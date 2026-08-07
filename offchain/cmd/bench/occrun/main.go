// OCC 压测 — 单账号预签名 + 并发 fire-and-forget
//
// 核心逻辑：
//   1. 预计算 N 个 nonce，预签名 N 笔 submitValidation
//   2. 开 N 个 goroutine，每个发 1 笔（raw HTTP POST，不等响应）
//   3. 全部发完后统一 poll receipt
//   4. 统计 success / abort / failed
//
// 对比两种场景：
//   - 全部写同一 agentId（V0 模拟，最大写冲突）
//   - 写不同 agentId（V1 模拟，分片降低冲突）
//
// 用法：
//   PRISM_EVALUATOR_KEY=<hex> go run . -n 50 -same
//   PRISM_EVALUATOR_KEY=<hex> go run . -n 50
package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rlp"
)

var (
	submitSel = common.Hex2Bytes("f17433c0")
)

type TxJob struct {
	RawHex string // 预签名好的 raw tx hex
	Hash   common.Hash
}

func main() {
	rpcURL := flag.String("rpc", "https://testnet-rpc.monad.xyz", "RPC URL")
	registry := flag.String("registry", "0x296d8DfDc0E306e3472a49CE5C9e0B7a68066881", "Registry contract")
	n := flag.Int("n", 500, "number of concurrent transactions")
	sameAgent := flag.Bool("same", false, "all txs to same agentId (max contention)")
	flag.Parse()

	privHex := os.Getenv("PRISM_EVALUATOR_KEY")
	keyEnv := os.Getenv("KEY")
	if keyEnv != "" {
		privHex = keyEnv
	}
	if privHex == "" {
		fmt.Fprintln(os.Stderr, "❌ PRISM_EVALUATOR_KEY 未设置")
		os.Exit(1)
	}
	privHex = strings.TrimPrefix(privHex, "0x")
	privKey, err := crypto.HexToECDSA(privHex)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 私钥解析失败: %v\n", err)
		os.Exit(1)
	}
	from := crypto.PubkeyToAddress(privKey.PublicKey)
	contract := common.HexToAddress(*registry)

	client, err := ethclient.Dial(*rpcURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ RPC 连接失败: %v\n", err)
		os.Exit(1)
	}

	chainID, _ := client.NetworkID(context.Background())
	nonce, err := client.PendingNonceAt(context.Background(), from)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 获取 nonce 失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Account:  %s\n", from.Hex())
	fmt.Printf("Nonce:    %d\n", nonce)
	fmt.Printf("ChainID:  %s\n", chainID.String())
	fmt.Printf("Contract: %s\n", contract.Hex())
	fmt.Printf("Concurrency: %d\n", *n)
	scenario := "分散 agentId"
	if *sameAgent {
		scenario = "全部同一 agentId"
	}
	fmt.Printf("Scenario: %s\n\n", scenario)

	gasPrice, _ := client.SuggestGasPrice(context.Background())
	if gasPrice == nil {
		gasPrice = big.NewInt(100_000_000_000)
	}
	gasLimit := uint64(200_000)
	signer := types.NewLondonSigner(chainID)

	// Step 1: 预签名所有交易
	fmt.Printf(">>> 预签名 %d 笔交易...\n", *n)
	jobs := make([]TxJob, *n)
	for i := 0; i < *n; i++ {
		var agentID uint64
		if *sameAgent {
			agentID = 0x1111 // all to same agent = max OCC contention
		} else {
			agentID = 0x1111 + uint64(i%50) // spread across agents
		}

		calldata := makeSubmitCalldata(agentID)
		tx := types.NewTransaction(nonce+uint64(i), contract, big.NewInt(0), gasLimit, gasPrice, calldata)
		signed, err := types.SignTx(tx, signer, privKey)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ 签名失败 #%d: %v\n", i, err)
			os.Exit(1)
		}
		raw, _ := rlp.EncodeToBytes(signed)
		jobs[i] = TxJob{
			RawHex: hex.EncodeToString(raw),
			Hash:   signed.Hash(),
		}
	}
	fmt.Printf("  ✅ 预签名完成\n")

	// Step 2: 并发发送
	fmt.Printf("\n>>> 并发发送 %d 笔...\n", *n)
	var sent, failed uint64
	client2 := &http.Client{Transport: &http.Transport{
		MaxConnsPerHost:     100,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     10 * time.Second,
	}}

	var wg sync.WaitGroup
	rateLimitRetries := &atomic.Uint64{}

	for i := 0; i < *n; i++ {
		wg.Add(1)
		go func(job TxJob, idx int) {
			defer wg.Done()

			payload := fmt.Sprintf(
				`{"jsonrpc":"2.0","method":"eth_sendRawTransaction","params":["0x%s"],"id":%d}`,
				job.RawHex, idx+1,
			)

			// 重试直到成功（应对 429）
			for attempt := 0; attempt < 10; attempt++ {
				req, _ := http.NewRequest("POST", *rpcURL, bytes.NewBufferString(payload))
				req.Header.Set("Content-Type", "application/json")
				resp, err := client2.Do(req)

				if err != nil {
					failed++
					time.Sleep(100 * time.Millisecond)
					break
				}
				resp.Body.Close()

				if resp.StatusCode == 429 {
					rateLimitRetries.Add(1)
					time.Sleep(time.Duration(50*(attempt+1)) * time.Millisecond)
					continue
				}
				sent++
				if idx < 10 || idx%50 == 0 {
					fmt.Printf("  ✅ #%d sent (%s...)\n", idx, job.Hash.Hex()[:14])
				}
				break
			}
		}(jobs[i], i)
	}
	wg.Wait()

	fmt.Printf("\n  Sent:  %d\n", sent)
	fmt.Printf("  Failed (network): %d\n", failed)
	fmt.Printf("  429 retries: %d\n", rateLimitRetries.Load())

	// Step 3: Poll receipt
	fmt.Printf("\n>>> 等待收据...\n")
	var (
		success   uint64
		abort     uint64
		finalFail uint64
		latencies []time.Duration
		mu        sync.Mutex
	)

	pollWg := sync.WaitGroup{}
	pollSem := make(chan struct{}, 20) // 最多 20 并发查询

	for _, job := range jobs {
		pollWg.Add(1)
		go func(job TxJob) {
			defer pollWg.Done()
			pollSem <- struct{}{}
			defer func() { <-pollSem }()

			start := time.Now()
		receipt, err := client.TransactionReceipt(context.Background(), job.Hash)
		for i := 0; i < 200 && (err != nil || receipt == nil); i++ {
			time.Sleep(500 * time.Millisecond)
			receipt, err = client.TransactionReceipt(context.Background(), job.Hash)
		}
			elapsed := time.Since(start)

			mu.Lock()
			latencies = append(latencies, elapsed)
			if err != nil {
				finalFail++
			} else if receipt.Status == 1 {
				success++
			} else {
				abort++
			}
			mu.Unlock()
		}(job)
	}
	pollWg.Wait()

	// Step 4: Report
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	total := success + abort + finalFail
	abortRate := float64(0)
	if sent > 0 {
		abortRate = float64(abort) / float64(sent) * 100
	}

	fmt.Printf("\n=== RESULT (%s) ===\n", scenario)
	fmt.Printf("  Total:    %d\n", total)
	fmt.Printf("  Success:  %d\n", success)
	fmt.Printf("  Abort:    %d (%.1f%%)\n", abort, abortRate)
	fmt.Printf("  NoReceipt:%d\n", finalFail)
	fmt.Printf("  P50/P95/P99: %s / %s / %s\n",
		formatDur(pctl(latencies, 50)),
		formatDur(pctl(latencies, 95)),
		formatDur(pctl(latencies, 99)),
	)
}

func makeSubmitCalldata(agentID uint64) []byte {
	d := make([]byte, 4)
	copy(d, submitSel)
	d = append(d, common.LeftPadBytes(big.NewInt(int64(agentID)).Bytes(), 32)...)
	score := new(big.Int).Mul(big.NewInt(8), new(big.Int).Exp(big.NewInt(10), big.NewInt(17), nil))
	scoreBytes := make([]byte, 32)
	score.FillBytes(scoreBytes)
	d = append(d, scoreBytes...)
	proofHash := crypto.Keccak256Hash([]byte("benchmark"))
	d = append(d, proofHash.Bytes()...)
	d = append(d, make([]byte, 32)...)
	d = append(d, common.LeftPadBytes([]byte{1}, 32)...)
	return d
}

func pctl(sorted []time.Duration, p int) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := len(sorted) * p / 100
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func formatDur(d time.Duration) string {
	if d == 0 {
		return "-"
	}
	if d > time.Second {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%dms", d.Milliseconds())
}
