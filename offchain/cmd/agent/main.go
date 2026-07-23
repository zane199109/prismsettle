// Package main implements the PrismSettle Agent binary (Phase 5).
//
// Modes:
//
//	default         Start the HTTP server for the named agent (--agent).
//	--register      Register the named agent with the Registry contract
//	                instead of starting the server. Exits after the tx is
//	                confirmed. Use this once per agent after deployment.
//
// Configuration:
//
//	--config        Path to agents.yaml (default: ../config/agents.yaml)
//	--agent         Agent name: defi | data_labeling | translation | eval
//	--agent-id      (register only) uint256 agentId to register as.
//	--rpc-url       (register only) Ethereum RPC URL.
//	--private-key   (register only) Hex private key (no 0x prefix) funding
//	                the registering wallet. Read from PRISM_REGISTER_KEY env
//	                var if flag is empty.
//
// API keys (LLM) are read from the env var named in agents.yaml (e.g.
// OPENAI_API_KEY); they are never stored in the YAML.
package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"flag"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/zane/web3-offchain/agents"
	"github.com/zane/web3-offchain/pkg/logger"
)

func main() {
	configPath := flag.String("config", "../config/agents.yaml", "path to agents.yaml")
	agentName := flag.String("agent", "", "agent name (defi | data_labeling | translation | eval)")
	register := flag.Bool("register", false, "register the agent with the Registry contract and exit")
	agentID := flag.Uint64("agent-id", 0, "(register only) uint256 agentId to register as")
	rpcURL := flag.String("rpc-url", "", "(register only) Ethereum RPC URL")
	privateKey := flag.String("private-key", "", "(register only) hex private key, no 0x prefix; reads PRISM_REGISTER_KEY if empty")
	flag.Parse()

	if *agentName == "" {
		fmt.Fprintln(os.Stderr, "error: --agent is required")
		os.Exit(2)
	}
	if _, err := agents.LoadConfig(*configPath); err != nil {
		fmt.Fprintf(os.Stderr, "load config failed: %v\n", err)
		os.Exit(1)
	}
	cfg, err := agents.CurrentConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config not loaded: %v\n", err)
		os.Exit(1)
	}
	logger.InitLogger()

	llm := agents.NewLLMClient(cfg.LLM.TimeoutSec)
	agent, err := buildAgent(*agentName, llm)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build agent: %v\n", err)
		os.Exit(1)
	}

	if *register {
		if err := runRegister(agent, *agentID, *rpcURL, *privateKey, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "register failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	port, err := cfg.Port(*agentName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "get port: %v\n", err)
		os.Exit(1)
	}
	srv := agents.NewServer(agent, llm)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := srv.Serve(ctx, port); err != nil {
		fmt.Fprintf(os.Stderr, "server exited: %v\n", err)
		os.Exit(1)
	}
}

// buildAgent instantiates the named agent.
func buildAgent(name string, llm *agents.LLMClient) (agents.Agent, error) {
	switch name {
	case "defi":
		return agents.NewDeFiAgent(llm), nil
	case "data_labeling":
		return agents.NewDataLabelingAgent(llm), nil
	case "translation":
		return agents.NewTranslationAgent(llm), nil
	case "eval":
		return agents.NewEvalAgent(llm), nil
	default:
		return nil, fmt.Errorf("unknown agent: %s", name)
	}
}

// runRegister calls Registry.registerAgent(agentId, metadata) on-chain.
// metadata is the JSON {"endpointUrl":"...","capabilities":"..."} expected
// by PrismSettleRegistry.sol.
func runRegister(agent agents.Agent, agentID uint64, rpcURL, privateKeyHex string, cfg *agents.AgentsConfig) error {
	if agentID == 0 {
		return fmt.Errorf("--agent-id is required for register mode")
	}
	if rpcURL == "" {
		return fmt.Errorf("--rpc-url is required for register mode")
	}
	if privateKeyHex == "" {
		privateKeyHex = os.Getenv("PRISM_REGISTER_KEY")
	}
	if privateKeyHex == "" {
		return fmt.Errorf("private key required: pass --private-key or set PRISM_REGISTER_KEY")
	}
	privateKeyHex = strings.TrimPrefix(privateKeyHex, "0x")

	keyBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return fmt.Errorf("decode private key: %w", err)
	}
	privKey, err := crypto.ToECDSA(keyBytes)
	if err != nil {
		return fmt.Errorf("parse private key: %w", err)
	}
	pubKey, ok := privKey.Public().(*ecdsa.PublicKey)
	if !ok {
		return fmt.Errorf("invalid public key type")
	}
	fromAddr := crypto.PubkeyToAddress(*pubKey)

	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return fmt.Errorf("dial rpc: %w", err)
	}
	defer client.Close()

	chainID, err := client.ChainID(context.Background())
	if err != nil {
		return fmt.Errorf("fetch chain id: %w", err)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(privKey, chainID)
	if err != nil {
		return fmt.Errorf("build signer: %w", err)
	}

	registryAddr := common.HexToAddress(cfg.Eval.RegistryAddress)
	registry, err := NewRegistryCaller(registryAddr, client)
	if err != nil {
		return fmt.Errorf("instantiate registry caller: %w", err)
	}

	md := agent.Metadata()
	metadataJSON := fmt.Sprintf(`{"endpointUrl":%q,"capabilities":%q}`, md.Endpoint, strings.Join(md.Capabilities, ","))
	logger.Info("registering agent",
		logger.String("agent", agent.Name()),
		logger.String("agent_id", fmt.Sprintf("%d", agentID)),
		logger.String("endpoint", md.Endpoint),
		logger.String("registry", registryAddr.Hex()),
		logger.String("from", fromAddr.Hex()))

	// Suggest gas price; tolerate legacy chains without 1559.
	gasTipCap, err := client.SuggestGasTipCap(context.Background())
	if err != nil {
		gasTipCap = big.NewInt(1_000_000_000) // 1 gwei fallback
	}
	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		gasPrice = big.NewInt(10_000_000_000)
	}
	auth.GasTipCap = gasTipCap
	auth.GasPrice = gasPrice
	auth.GasLimit = 300_000

	tx, err := registry.RegisterAgent(auth, new(big.Int).SetUint64(agentID), metadataJSON)
	if err != nil {
		return fmt.Errorf("send registerAgent tx: %w", err)
	}
	logger.Info("registerAgent tx sent", logger.String("tx_hash", tx.Hash().Hex()))

	// Wait for confirmation (1 confirmation; dev/anvil is fine).
	waitCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	receipt, err := bind.WaitMined(waitCtx, client, tx)
	if err != nil {
		return fmt.Errorf("wait for receipt: %w", err)
	}
	if receipt.Status != 1 {
		return fmt.Errorf("registerAgent tx reverted: %s (status=%d)", tx.Hash().Hex(), receipt.Status)
	}
	logger.Info("agent registered on-chain",
		logger.String("agent", agent.Name()),
		logger.String("agent_id", fmt.Sprintf("%d", agentID)),
		logger.String("tx_hash", tx.Hash().Hex()),
		logger.Uint64("block", receipt.BlockNumber.Uint64()))
	return nil
}
