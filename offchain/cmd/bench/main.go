// Package main implements the V0/V1 OCC abort-rate benchmark.
//
// 这是 Phase 3 任务 3.4a 的压测脚本骨架。实际压测执行延后到 Phase 9
// （anvil + Go 环境就绪后），符合"决策2 优先级后调"原则。
//
// 用法（Phase 9 就绪后）：
//
//	go run ./cmd/bench/ \
//	  -rpc http://localhost:8545 \
//	  -v0 0xV0RegistryAddress \
//	  -v1 0xV1RegistryAddress \
//	  -concurrency 500 \
//	  -duration 60s
//
// 输出：
//   - V0/V1 对照表（abort rate、throughput、latency p50/p95/p99）
//   - 三项约束声明（FR-T06）
//   - JSON 报告文件（供压测报告引用）
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math/big"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// Config 压测配置
type Config struct {
	RPCEndpoint  string        // anvil/Monad testnet RPC URL
	V0Address    string        // BaselineRegistry 地址
	V1Address    string        // PrismSettleRegistry 地址
	Concurrency  int           // 并发数（默认 500，对齐 NFR-MN01）
	Duration     time.Duration // 压测时长（默认 60s）
	ValidatorKey string        // Validator 私钥（用于签名 tx）
	AgentIDs     []uint64      // 目标 agentId 列表（默认 0x1111~0x1111+50）
	OutputFile   string        // JSON 报告输出路径
}

// Result 单次压测结果
type Result struct {
	TotalSent     uint64        // 发送总数
	TotalSuccess  uint64        // 成功总数
	TotalAbort    uint64        // OCC abort 总数（tx 失败 retry）
	TotalFailed   uint64        // 其他失败总数
	ThroughputTPS float64       // 每秒成功 tx 数
	LatencyP50    time.Duration // 中位延迟
	LatencyP95    time.Duration // p95 延迟
	LatencyP99    time.Duration // p99 延迟
	AbortRate     float64       // abort rate = TotalAbort / TotalSent
}

// BenchmarkReport 完整压测报告（对齐 FR-T03/T04/T06）
type BenchmarkReport struct {
	Timestamp   string   `json:"timestamp"`
	Constraints []string `json:"constraints"` // FR-T06 三项约束声明
	Config      Config   `json:"config"`
	V0Result    Result   `json:"v0_result"`
	V1Result    Result   `json:"v1_result"`
	Conclusion  string   `json:"conclusion"` // V1 abort rate < 5% (NFR-MN01)
}

func main() {
	cfg := parseFlags()

	// FR-T06 三项约束声明（报告内显式声明）
	constraints := []string{
		"1. 统一环境：同一节点/RPC、服务器硬件、数据库配置、区块参数、Gas 费率",
		"2. 统一压测工具&用例：固定脚本、并发量级、请求模板、执行轮次、样本总量",
		"3. 前置数据清零：每次测试前重置合约/业务状态",
	}

	fmt.Println("=== PrismSettle V0/V1 OCC Abort Rate Benchmark ===")
	fmt.Println("Constraints (FR-T06):")
	for _, c := range constraints {
		fmt.Println("  " + c)
	}
	fmt.Printf("\nConfig: concurrency=%d, duration=%s\n", cfg.Concurrency, cfg.Duration)
	fmt.Printf("V0: %s\n", cfg.V0Address)
	fmt.Printf("V1: %s\n\n", cfg.V1Address)

	// 连接 RPC
	client, err := ethclient.Dial(cfg.RPCEndpoint)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ RPC 连接失败: %v\n", err)
		fmt.Fprintf(os.Stderr, "   提示：Phase 9 anvil 环境就绪后再执行实际压测\n")
		os.Exit(1)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Duration*2)
	defer cancel()

	// 压测 V0
	fmt.Println(">>> 压测 V0 (BaselineRegistry, 单槽存储)...")
	v0Result := runBenchmark(ctx, client, cfg, common.HexToAddress(cfg.V0Address))
	printResult("V0", v0Result)

	// 前置数据清零（约束 3）—— Phase 9 实现实际的 reset 逻辑
	fmt.Println("\n>>> 前置数据清零（FR-T06 约束 3）...")
	resetContracts(client, cfg)

	// 压测 V1
	fmt.Println(">>> 压测 V1 (PrismSettleRegistry, 256 分片存储)...")
	v1Result := runBenchmark(ctx, client, cfg, common.HexToAddress(cfg.V1Address))
	printResult("V1", v1Result)

	// 生成报告
	conclusion := "PASS"
	if v1Result.AbortRate >= 0.05 {
		conclusion = fmt.Sprintf("FAIL: V1 abort rate %.2f%% >= 5%% (NFR-MN01)", v1Result.AbortRate*100)
	} else {
		conclusion = fmt.Sprintf("PASS: V1 abort rate %.2f%% < 5%% (NFR-MN01)", v1Result.AbortRate*100)
	}

	report := BenchmarkReport{
		Timestamp:   time.Now().Format(time.RFC3339),
		Constraints: constraints,
		Config:      cfg,
		V0Result:    v0Result,
		V1Result:    v1Result,
		Conclusion:  conclusion,
	}

	// 输出 JSON 报告
	if cfg.OutputFile != "" {
		data, _ := json.MarshalIndent(report, "", "  ")
		if err := os.WriteFile(cfg.OutputFile, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️ 报告写入失败: %v\n", err)
		} else {
			fmt.Printf("\n📄 报告已写入: %s\n", cfg.OutputFile)
		}
	}

	fmt.Println("\n=== Conclusion ===")
	fmt.Println(conclusion)
}

// parseFlags 解析命令行参数
func parseFlags() Config {
	cfg := Config{}
	flag.StringVar(&cfg.RPCEndpoint, "rpc", "http://localhost:8545", "RPC endpoint")
	flag.StringVar(&cfg.V0Address, "v0", "", "V0 BaselineRegistry address")
	flag.StringVar(&cfg.V1Address, "v1", "", "V1 PrismSettleRegistry address")
	flag.IntVar(&cfg.Concurrency, "concurrency", 500, "concurrent senders (NFR-MN01)")
	flag.DurationVar(&cfg.Duration, "duration", 60*time.Second, "benchmark duration")
	flag.StringVar(&cfg.ValidatorKey, "validator-key", "", "validator private key (hex)")
	flag.StringVar(&cfg.OutputFile, "output", "bench_report.json", "JSON report output path")
	flag.Parse()

	if cfg.V0Address == "" || cfg.V1Address == "" {
		fmt.Fprintln(os.Stderr, "❌ 必须指定 -v0 和 -v1 合约地址")
		os.Exit(1)
	}

	// 默认 agentId 列表：0x1111 ~ 0x1111+50（覆盖多个分片）
	for i := uint64(0); i < 50; i++ {
		cfg.AgentIDs = append(cfg.AgentIDs, 0x1111+i)
	}
	return cfg
}

// runBenchmark 执行压测：并发发送 submitValidation tx，统计 abort rate
func runBenchmark(ctx context.Context, client *ethclient.Client, cfg Config, registry common.Address) Result {
	var (
		totalSent    uint64
		totalSuccess uint64
		totalAbort   uint64
		totalFailed  uint64
		latencies    sync.Map // goroutine-safe latency 收集
	)

	// 压测截止时间
	deadline := time.Now().Add(cfg.Duration)
	var wg sync.WaitGroup

	// 启动 N 个并发 sender
	for i := 0; i < cfg.Concurrency; i++ {
		wg.Add(1)
		go func(senderID int) {
			defer wg.Done()
			for time.Now().Before(deadline) {
				agentID := cfg.AgentIDs[senderID%len(cfg.AgentIDs)]
				atomic.AddUint64(&totalSent, 1)

				start := time.Now()
				// TODO Phase 9: 实际发送 submitValidation tx
				// 1. 构造 calldata: submitValidation(agentId, 0.8e18, proofHash, 0, 0)
				// 2. 签名 + eth_sendRawTransaction
				// 3. 等待 receipt，判断 success/abort/failed
				//    - receipt.Status == 1 → success
				//    - receipt.Status == 0 → abort（OCC 冲突）
				//    - err != nil → failed
				_ = agentID
				_ = registry
				elapsed := time.Since(start)

				// 占位：实际压测时由 receipt 决定
				// 当前骨架仅记录 latency，不实际发包
				latencies.Store(senderID, elapsed)

				// Phase 9 实际逻辑示例（伪代码）：
				// receipt, err := sendSubmitValidation(client, cfg.ValidatorKey, registry, agentID)
				// if err != nil { atomic.AddUint64(&totalFailed, 1); continue }
				// if receipt.Status == 0 { atomic.AddUint64(&totalAbort, 1); continue }
				// atomic.AddUint64(&totalSuccess, 1)
			}
		}(i)
	}
	wg.Wait()

	// 计算吞吐与延迟分位数
	latencyList := collectLatencies(&latencies)
	result := Result{
		TotalSent:    atomic.LoadUint64(&totalSent),
		TotalSuccess: atomic.LoadUint64(&totalSuccess),
		TotalAbort:   atomic.LoadUint64(&totalAbort),
		TotalFailed:  atomic.LoadUint64(&totalFailed),
		LatencyP50:   percentile(latencyList, 50),
		LatencyP95:   percentile(latencyList, 95),
		LatencyP99:   percentile(latencyList, 99),
	}
	if cfg.Duration > 0 {
		result.ThroughputTPS = float64(result.TotalSuccess) / cfg.Duration.Seconds()
	}
	if result.TotalSent > 0 {
		result.AbortRate = float64(result.TotalAbort) / float64(result.TotalSent)
	}
	return result
}

// resetContracts 前置数据清零（FR-T06 约束 3）
// Phase 9 实现：重新部署 V0/V1 合约，或调用 reset 函数
func resetContracts(client *ethclient.Client, cfg Config) {
	// TODO Phase 9:
	// 1. 重新部署 V0 + V1 合约（状态清零）
	// 2. 或调用 admin reset 函数（如果有）
	// 3. Validator 重新质押
	fmt.Println("   [TODO Phase 9] 实际重置合约状态")
	_ = new(big.Int) // 占位引用 big 包
}

// collectLatencies 从 sync.Map 收集延迟数据
func collectLatencies(m *sync.Map) []time.Duration {
	var list []time.Duration
	m.Range(func(_, v interface{}) bool {
		list = append(list, v.(time.Duration))
		return true
	})
	return list
}

// percentile 计算分位数
func percentile(list []time.Duration, p int) time.Duration {
	if len(list) == 0 {
		return 0
	}
	// 简化实现：实际应排序后取分位
	// Phase 9 可用 sort + 索引计算
	if p >= 100 {
		return list[len(list)-1]
	}
	idx := len(list) * p / 100
	if idx >= len(list) {
		idx = len(list) - 1
	}
	return list[idx]
}

// printResult 打印压测结果
func printResult(label string, r Result) {
	fmt.Printf("\n--- %s Result ---\n", label)
	fmt.Printf("Total Sent:    %d\n", r.TotalSent)
	fmt.Printf("Total Success: %d\n", r.TotalSuccess)
	fmt.Printf("Total Abort:   %d (OCC conflict)\n", r.TotalAbort)
	fmt.Printf("Total Failed:  %d\n", r.TotalFailed)
	fmt.Printf("Throughput:    %.2f TPS\n", r.ThroughputTPS)
	fmt.Printf("Abort Rate:    %.2f%%\n", r.AbortRate*100)
	fmt.Printf("Latency P50:   %v\n", r.LatencyP50)
	fmt.Printf("Latency P95:   %v\n", r.LatencyP95)
	fmt.Printf("Latency P99:   %v\n", r.LatencyP99)
}
