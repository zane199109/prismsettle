// Benchmark 使用 N 个独立账号并发提交 submitValidation，对比
// V0 (BaselineRegistry, 单槽) 和 V1 (PrismSettleRegistry, 256-shard) 的 OCC abort rate。
//
// 流程：
//   1. 生成 N 个临时密钥对（bench worker）
//   2. deployer 给每个 worker 转 0.01 MON + 授权 REGISTRY_EVALUATOR_ROLE
//   3. 每个 worker 发 1 笔 submitValidation（同时发出）
//   4. 等所有收据，统计成功/失败/中止
//   5. V0 跑完跑 V1，报告对比
//
// 用法：
//   cd offchain && DEPLOYER_KEY=<hex> PRISM_EVALUATOR_KEY=<hex> go run ./cmd/bench/
package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"flag"
	"fmt"
	"math/big"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

type Worker struct {
	Key    *ecdsa.PrivateKey
	Addr   common.Address
	AgentID uint64
}

type Result struct {
	TotalSent    uint64  `json:"total_sent"`
	TotalSuccess uint64  `json:"total_success"`
	TotalAbort   uint64  `json:"total_abort"`
	TotalFailed  uint64  `json:"total_failed"`
	AbortRate    float64 `json:"abort_rate"`
	Throughput   float64 `json:"throughput_tps"`
	LatencyP50   string  `json:"latency_p50"`
	LatencyP95   string  `json:"latency_p95"`
	LatencyP99   string  `json:"latency_p99"`
}

type Report struct {
	Timestamp   string   `json:"timestamp"`
	Concurrency int      `json:"concurrency"`
	V0          *Result  `json:"v0,omitempty"`
	V1          *Result  `json:"v1,omitempty"`
}

var (
	submitValidationSig = common.Hex2Bytes("f17433c0")
	registerAgentSig    = common.Hex2Bytes("8b3a")
	grantRoleSig        = common.Hex2Bytes("2f2ff15d")
)

func main() {
	rpc := flag.String("rpc", "https://testnet-rpc.monad.xyz", "RPC URL")
	v0Addr := flag.String("v0", "0xAecf336B8C5470E0c626ed7CE3aA682A9bBaa1d8", "V0 BaselineRegistry")
	v1Addr := flag.String("v1", "0x296d8DfDc0E306e3472a49CE5C9e0B7a68066881", "V1 PrismSettleRegistry")
	nWorkers := flag.Int("n", 50, "number of benchmark workers (accounts)")
	output := flag.String("output", "bench_report.json", "JSON report path")
	flag.Parse()

	deployerHex := os.Getenv("DEPLOYER_KEY")
	if deployerHex == "" {
		fmt.Fprintln(os.Stderr, "❌ DEPLOYER_KEY 未设置")
		os.Exit(1)
	}
	evalHex := os.Getenv("PRISM_EVALUATOR_KEY")
	if evalHex == "" {
		evalHex = deployerHex
	}

	ctx := context.Background()
	client, err := ethclient.Dial(*rpc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ RPC 连接失败: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	v0 := common.HexToAddress(*v0Addr)
	v1 := common.HexToAddress(*v1Addr)
	chainID, _ := client.NetworkID(ctx)

	// Step 1: 生成 N 个 worker
	fmt.Printf(">>> 生成 %d 个 worker...\n", *nWorkers)
	workers := make([]Worker, *nWorkers)
	for i := range workers {
		key, _ := crypto.GenerateKey()
		workers[i] = Worker{
			Key:     key,
			Addr:    crypto.PubkeyToAddress(key.PublicKey),
			AgentID: 0x1111 + uint64(i),
		}
	}

	// Step 2: deployer 注册 agent + 转账 + 授权
	fmt.Println(">>> 注册 agent + 转账 + 授权...")
	deployerKey := hexToKey(deployerHex)
	deployerAddr := crypto.PubkeyToAddress(deployerKey.PublicKey)

	gasPrice, _ := client.SuggestGasPrice(ctx)
	if gasPrice == nil {
		gasPrice = big.NewInt(100_000_000_000)
	}

	// 给 agentId 范围注册
	nonce, _ := client.PendingNonceAt(ctx, deployerAddr)
	fee := new(big.Int).Mul(gasPrice, big.NewInt(2_000_000)) // 0.2 MON at 100 gwei

	for i, w := range workers {
		// grant role on V0 (agents already registered from previous runs)
			sendTx(ctx, client, deployerKey, deployerAddr, nonce, v0, big.NewInt(0), 80000, gasPrice, makeGrantRoleCalldata(evalRoleHash(), w.Addr))
			nonce++
			// grant role on V1
			sendTx(ctx, client, deployerKey, deployerAddr, nonce, v1, big.NewInt(0), 80000, gasPrice, makeGrantRoleCalldata(evalRoleHash(), w.Addr))
			nonce++
			// fund worker
			sendTx(ctx, client, deployerKey, deployerAddr, nonce, w.Addr, fee, 21000, gasPrice, nil)
		nonce++

		if (i+1)%10 == 0 {
			fmt.Printf("  %d/%d workers ready\n", i+1, *nWorkers)
		}
	}

	// 等待所有 setup tx 确认
	fmt.Println(">>> 等待 setup 交易确认...")
	for {
		current, _ := client.PendingNonceAt(ctx, deployerAddr)
		if current >= nonce {
			break
		}
		fmt.Printf("  deployer nonce: %d/%d\n", current, nonce)
		time.Sleep(5 * time.Second)
	}
	fmt.Println("  ✅ setup 完成")

	// Step 3+4: 跑 V0 压测
	fmt.Println("\n>>> 压测 V0 (单槽)...")
	v0Result := runBenchWorkers(ctx, client, chainID, v0, workers, gasPrice, evalHex)

	// 等 nonce 重置
	time.Sleep(5 * time.Second)

	// Step 5: 跑 V1 压测
	fmt.Println("\n>>> 压测 V1 (256-shard)...")
	v1Result := runBenchWorkers(ctx, client, chainID, v1, workers, gasPrice, evalHex)

	// Step 6: 出报告
	printResult("V0", v0Result)
	printResult("V1", v1Result)

	report := Report{
		Timestamp:   time.Now().Format(time.RFC3339),
		Concurrency: *nWorkers,
		V0:          v0Result,
		V1:          v1Result,
	}
	data, _ := json.MarshalIndent(report, "", "  ")
	os.WriteFile(*output, data, 0644)
	fmt.Printf("\n📄 %s\n", *output)
	fmt.Println(string(data))
}

// runBenchWorkers N 个 worker 并发各发 1 笔 submitValidation
func runBenchWorkers(ctx context.Context, client *ethclient.Client, chainID *big.Int,
	registry common.Address, workers []Worker, gasPrice *big.Int, evalKeyHex string) *Result {

	start := time.Now()
	var (
		mu       sync.Mutex
		success  uint64
		abort    uint64
		failed   uint64
		sent     uint64
		latencyMs []float64
	)

	var wg sync.WaitGroup

	// Stagger workers to avoid RPC rate limit (50 req/s)
	sem := make(chan struct{}, 10) // max 10 concurrent RPC calls

	for _, w := range workers {
		wg.Add(1)
		go func(w Worker) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
		tStart := time.Now()

		// Check balance first
			bal := retryBalance(ctx, client, w.Addr)
			if bal == nil || bal.Cmp(big.NewInt(0)) == 0 {
				mu.Lock()
				fmt.Printf("  ❌ worker 0x%x no balance\n", w.AgentID)
				failed++
				mu.Unlock()
				return
			}

			// Check role on contract
			roleData := makeHasRoleCalldata(evalRoleHash(), w.Addr)
			roleResult, _ := client.CallContract(ctx, ethereum.CallMsg{
				To:   &registry,
				Data: roleData,
			}, nil)
			hasRole := len(roleResult) == 32 && roleResult[31] == 1
			if !hasRole {
				mu.Lock()
				fmt.Printf("  ❌ worker 0x%x no role on %s\n", w.AgentID, registry.Hex()[2:6])
				failed++
				mu.Unlock()
				return
			}

			nonce := retryNonce(ctx, client, w.Addr)
			if nonce == nil {
				mu.Lock()
				failed++
				mu.Unlock()
				return
			}

			calldata, err := makeSubmitCalldata(w.AgentID)
			if err != nil {
				mu.Lock()
				failed++
				mu.Unlock()
				return
			}

			tx := types.NewTransaction(*nonce, registry, big.NewInt(0), 200_000, gasPrice, calldata)
			signer := types.NewLondonSigner(chainID)
			signed, err := types.SignTx(tx, signer, w.Key)
			if err != nil {
				mu.Lock()
				failed++
				mu.Unlock()
				return
			}

			err = retrySend(ctx, client, signed)
			if err != nil {
				mu.Lock()
				failed++
				mu.Unlock()
				return
			}
			mu.Lock()
			sent++
			mu.Unlock()

			// 等收据
			var receipt *types.Receipt
			for i := 0; i < 60; i++ {
				receipt, _ = client.TransactionReceipt(ctx, signed.Hash())
				if receipt != nil {
					break
				}
				time.Sleep(500 * time.Millisecond)
			}

			elapsed := time.Since(tStart)
			mu.Lock()
			latencyMs = append(latencyMs, float64(elapsed.Milliseconds()))

			if receipt == nil {
				failed++
			} else if receipt.Status == 1 {
				success++
			} else {
				abort++
			}
			mu.Unlock()
		}(w)
	}
	wg.Wait()

	duration := time.Since(start).Seconds()
	sort.Float64s(latencyMs)

	r := &Result{
		TotalSent:   sent,
		TotalSuccess: success,
		TotalAbort:  abort,
		TotalFailed: failed,
		Throughput:  float64(success) / duration,
	}
	if sent > 0 {
		r.AbortRate = float64(abort) / float64(sent)
	}
	if len(latencyMs) > 0 {
		r.LatencyP50 = fmt.Sprintf("%.0fms", pctl(latencyMs, 50))
		r.LatencyP95 = fmt.Sprintf("%.0fms", pctl(latencyMs, 95))
		r.LatencyP99 = fmt.Sprintf("%.0fms", pctl(latencyMs, 99))
	}
	return r
}

// sendTx 签名并发送交易，不等待收据
func sendTx(ctx context.Context, client *ethclient.Client, key *ecdsa.PrivateKey, from common.Address,
	nonce uint64, to common.Address, value *big.Int, gas uint64, gasPrice *big.Int, data []byte) {

	signer := types.NewLondonSigner(client_chainID(ctx, client))
	tx := types.NewTransaction(nonce, to, value, gas, gasPrice, data)
	signed, _ := types.SignTx(tx, signer, key)
	client.SendTransaction(ctx, signed)
}

func makeRegisterCalldata(agentID uint64) []byte {
	meta := `{"endpointUrl":"http://bench","capabilities":"bench"}`
	d := make([]byte, 4)
	copy(d, registerAgentSig)
	// agentId (uint256)
	d = append(d, common.LeftPadBytes(big.NewInt(int64(agentID)).Bytes(), 32)...)
	// string offset = 0x40
	d = append(d, common.LeftPadBytes(big.NewInt(64).Bytes(), 32)...)
	// string length
	d = append(d, common.LeftPadBytes(big.NewInt(int64(len(meta))).Bytes(), 32)...)
	// string data
	d = append(d, []byte(meta)...)
	d = append(d, make([]byte, 32-len(meta)%32)...)
	return d
}

func makeGrantRoleCalldata(role common.Hash, addr common.Address) []byte {
	d := make([]byte, 4)
	copy(d, grantRoleSig)
	d = append(d, role.Bytes()...)
	d = append(d, common.LeftPadBytes(addr.Bytes(), 32)...)
	return d
}

func makeHasRoleCalldata(role common.Hash, addr common.Address) []byte {
	d := common.Hex2Bytes("91d14854")
	d = append(d, role.Bytes()...)
	d = append(d, common.LeftPadBytes(addr.Bytes(), 32)...)
	return d
}

func makeSubmitCalldata(agentID uint64) ([]byte, error) {
	d := make([]byte, 4)
	copy(d, submitValidationSig)

	// agentId (uint256)
	d = append(d, common.LeftPadBytes(big.NewInt(int64(agentID)).Bytes(), 32)...)

	// score = 0.8e18 (uint96)
	score := new(big.Int).Mul(big.NewInt(8), new(big.Int).Exp(big.NewInt(10), big.NewInt(17), nil))
	scoreBytes := make([]byte, 32)
	score.FillBytes(scoreBytes)
	d = append(d, scoreBytes...)

	// proofHash (bytes32)
	proofHash := crypto.Keccak256Hash([]byte("benchmark"))
	d = append(d, proofHash.Bytes()...)

	// jobId (uint256) = 0
	d = append(d, make([]byte, 32)...)

	// source (uint8) = 1
	d = append(d, common.LeftPadBytes([]byte{1}, 32)...)

	return d, nil
}

func evalRoleHash() common.Hash {
	return crypto.Keccak256Hash([]byte("REGISTRY_EVALUATOR_ROLE"))
}

func hexToKey(hex string) *ecdsa.PrivateKey {
	hex = strings.TrimPrefix(hex, "0x")
	key, err := crypto.HexToECDSA(hex)
	if err != nil {
		panic("invalid private key: " + err.Error())
	}
	return key
}

func client_chainID(ctx context.Context, client *ethclient.Client) *big.Int {
	id, _ := client.NetworkID(ctx)
	return id
}

func printResult(label string, r *Result) {
	fmt.Printf("\n--- %s ---\n", label)
	fmt.Printf("  Sent:     %d\n", r.TotalSent)
	fmt.Printf("  Success:  %d\n", r.TotalSuccess)
	fmt.Printf("  Abort:    %d\n", r.TotalAbort)
	fmt.Printf("  Failed:   %d\n", r.TotalFailed)
	fmt.Printf("  Abort%%:   %.2f%%\n", r.AbortRate*100)
	fmt.Printf("  TPS:      %.2f\n", r.Throughput)
	fmt.Printf("  P50/P95/P99: %s / %s / %s\n", r.LatencyP50, r.LatencyP95, r.LatencyP99)
}

func pctl(sorted []float64, p int) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := len(sorted) * p / 100
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// retryBalance 带重试的余额查询（应对 RPC 429）
func retryBalance(ctx context.Context, client *ethclient.Client, addr common.Address) *big.Int {
	for i := 0; i < 5; i++ {
		bal, err := client.BalanceAt(ctx, addr, nil)
		if err == nil {
			return bal
		}
		time.Sleep(time.Duration(200*(i+1)) * time.Millisecond)
	}
	return nil
}

// retryNonce 带重试的 nonce 查询
func retryNonce(ctx context.Context, client *ethclient.Client, addr common.Address) *uint64 {
	for i := 0; i < 5; i++ {
		n, err := client.PendingNonceAt(ctx, addr)
		if err == nil {
			return &n
		}
		time.Sleep(time.Duration(200*(i+1)) * time.Millisecond)
	}
	return nil
}

// retrySend 带重试的交易发送
func retrySend(ctx context.Context, client *ethclient.Client, tx *types.Transaction) error {
	for i := 0; i < 5; i++ {
		err := client.SendTransaction(ctx, tx)
		if err == nil {
			return nil
		}
		time.Sleep(time.Duration(200*(i+1)) * time.Millisecond)
	}
	return fmt.Errorf("send failed after 5 retries")
}
