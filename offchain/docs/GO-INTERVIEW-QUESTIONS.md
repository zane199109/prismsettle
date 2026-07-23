## Part 1: Go Language Basics (Go语言基础)

### Q1: Goroutine vs Thread — Goroutine和线程的区别？

**Answer / 参考答案:**

1. **Lightweight / 轻量级**: Goroutines start with ~2KB stack; threads need 1-8MB.
   Goroutine初始栈约2KB；线程需要1-8MB栈空间。

2. **Scheduling / 调度方式**:
   - Threads: Kernel-level scheduling, managed by OS.
     线程：内核级调度，由操作系统管理。
   - Goroutines: M:N model, scheduled by Go runtime (GMP model).
     Goroutine：M:N模型，由Go runtime调度（GMP模型：G=Goroutine, M=Machine/OS线程, P=Processor/局部调度器）。

3. **Creation Cost / 创建成本**: Goroutines are extremely cheap to create/destroy; threads are expensive.
   Goroutine创建/销毁成本极低；线程成本较高。

4. **Blocking Impact / 阻塞影响**: A blocked goroutine does not block other goroutines; a blocked thread blocks its entire OS thread.
   Goroutine阻塞不影响其他Goroutine；线程阻塞会影响整个OS线程。

**GMP Model / GMP模型简述:**
- **G**: Goroutine, contains stack and execution state.
  Goroutine，包含栈和执行状态。
- **M**: Machine, OS thread that executes G's code.
  OS线程，执行G的代码。
- **P**: Processor, local scheduler that maintains G queues.
  局部调度器，维护G队列。
- G migrates between Ps via work-stealing.
  G通过work-stealing机制在P之间迁移。

---

### Q2: Channel — Channel底层实现原理？

**Answer / 参考答案:**

1. **Data Structure / 数据结构**: `hchan` struct with circular buffer (buf), send/receive pointers, element size, lock.
   hchan结构体，包含环形缓冲区（buf）、队列头尾指针、元素大小、锁等。

2. **Unbuffered Channel / 无缓冲channel**: Send and receive must pair up; otherwise block.
   send和recv必须配对，否则阻塞。

3. **Buffered Channel / 有缓冲channel**: Circular buffer with capacity; send blocks when full, recv blocks when empty.
   环形缓冲区带容量；满时send阻塞，空时recv阻塞。

4. **Close Channel / 关闭channel**: `close(ch)` — receivers get zero value and `ok = false`.
   close(ch) — 接收方得到零值和ok=false。

5. **Deadlock Detection / 死锁检测**: Go runtime panics on deadlock (all goroutines asleep).
   Go runtime在所有goroutine阻塞时panic。

**Your Project / 你的项目:**
- web3-offchain: Channels for RPC (Remote Procedure Call) health check results and SSE (Server-Sent Events) message dispatching.
  web3-offchain中用channel传递RPC健康检查结果和SSE消息分发。

---

### Q3: Context — Context的作用和使用场景？

**Answer / 参考答案:**

**Purpose / 作用**: Propagate cancellation signals, timeouts, and request-scoped values across goroutine trees.
传播取消信号、超时控制和请求范围的值。

**Usage Scenarios / 使用场景:**

1. **Timeout Control / 超时控制**: `context.WithTimeout` — Set deadline for RPC calls.
   为RPC调用设置超时。

2. **Cancellation Signal / 取消信号**: `context.WithCancel` — Notify all goroutines to exit on graceful shutdown.
   优雅关闭时通知所有goroutine退出。

3. **Request-scoped Values / 请求范围的值**: `context.WithValue` — Pass request ID, user info, etc.
   传递request ID、用户信息等。

**Your Project / 你的项目:**
- web3-offchain: Context controls RPC client timeouts and graceful shutdown.
  web3-offchain中context控制RPC客户端超时和优雅关闭。
- AEP: Context manages SSE connection lifecycle.
  AEP中context管理SSE连接生命周期。

---

### Q4: Mutex vs RWMutex — sync.Mutex和sync.RWMutex的区别？

**Answer / 参考答案:**

| Feature / 特性 | sync.Mutex | sync.RWMutex |
|----------------|------------|--------------|
| Readers / 读者 | One at a time / 同时只能一个 | Multiple allowed / 允许多个同时读 |
| Writers / 写者 | Exclusive / 独占 | Exclusive / 独占 |
| Best For / 适用场景 | Write-heavy / 写多读少 | Read-heavy / 读多写少 |
| Methods / 方法 | Lock(), Unlock() | Lock(), Unlock(), RLock(), RUnlock() |

**Your Project / 你的项目:**
- web3-offchain: RWMutex protects RPC node pool (frequent reads, rare updates).
  web3-offchain中RWMutex保护RPC节点池（频繁读，偶尔更新）。

---

### Q5: Go GC Principle — Go的GC原理？

**Answer / 参考答案:**

1. **Tri-color Mark-and-Sweep / 三色标记清除法**:
   - White: Unmarked, possibly collected.
     白色：未被标记，可能被回收。
   - Gray: Marked, but its referenced objects not scanned.
     灰色：已被标记，但其引用对象未扫描。
   - Black: Marked, and its referenced objects scanned.
     黑色：已被标记，且其引用对象已扫描。

2. **Write Barrier / 写屏障**: Ensures correctness during concurrent marking.
   保证并发标记期间的正确性。

- Mixed Write Barrier / 混合写屏障** (Go 1.8+): Reduces STW (Stop-The-World, 全局停顿时间) 时间.
     减少STW（Stop-The-World，全局停顿时间）时间。

4. **Trigger Condition / 触发条件**: When allocated memory reaches threshold.
   分配的内存达到阈值时触发。

- Marking is concurrent; only mark start and mark stop need STW (Stop-The-World, 全局停顿).
     标记阶段是并发的，只有标记开始和标记终止需要STW（Stop-The-World，全局停顿）。

**Your Project / 你的项目:**
- web3-offchain: Batch inserts reduce memory allocation; sync.Pool for object reuse.
  web3-offchain中批量写入减少内存分配；sync.Pool复用对象减少GC压力。

---

### Q6: Interface — Interface底层实现原理？

**Answer / 参考答案:**

An interface in Go is a **two-word structure**:
Go的interface是一个**双字结构**：

1. **Type / 类型信息**: Pointer to type descriptor (name, methods, size).
   指向类型描述符的指针（名称、方法、大小）。

2. **Data / 数据指针**: Pointer to the actual value.
   指向实际值的指针。

**Dynamic Dispatch / 动态分发:**
- When calling an interface method, Go looks up the method in the type descriptor.
  调用接口方法时，Go在类型描述符中查找方法。
- This is slower than direct method calls but enables polymorphism.
  比直接方法调用慢，但实现了多态。

**Empty Interface / 空接口:**
- `interface{}` (or `any` in Go 1.18+) stores any value.
  interface{}（或Go 1.18+的any）存储任意值。
- Under the hood, it still holds type + data pointers.
  底层仍然是类型+数据指针。

**Your Project / 你的项目:**
- web3-offchain: `interface{}` used in plugin registry for generic event handlers.
  web3-offchain中interface{}用于插件注册表的通用事件处理器。

---

### Q7: Slice vs Array — Slice和Array的区别？

**Answer / 参考答案:**

| Feature / 特性 | Array / 数组 | Slice / 切片 |
|----------------|-------------|-------------|
| Size / 大小 | Fixed / 固定 | Dynamic / 动态 |
| Type / 类型 | Value type / 值类型 | Reference type / 引用类型 |
| Underlying / 底层 | Direct memory storage | Header: ptr, len, cap |
| Passing / 传递 | Copies entire array | Shares underlying array |

**Slice Header / 切片头部:**
```
struct {
    ptr *byte   // Pointer to underlying array
    len int     // Length
    cap int     // Capacity
}
```

**Common Pitfall / 常见陷阱:**
- Modifying a slice element modifies the underlying array — affects all slices sharing it.
  修改切片元素会修改底层数组——影响所有共享该数组的切片。
- Use `make([]T, len, cap)` to allocate with specific capacity.
  使用make([]T, len, cap)分配特定容量的切片。

**Your Project / 你的项目:**
- web3-offchain: Slices for batch event collection before database insertion.
  web3-offchain中用切片批量收集事件后插入数据库。

---

### Q8: Map Thread Safety — Map线程安全吗？

**Answer / 参考答案:**

**No, regular maps are NOT thread-safe in Go.**
**普通map在Go中不是线程安全的。**

- Concurrent read + write causes panic: "concurrent map reads and map write".
  并发读写会导致panic："concurrent map reads and map write"。

**Solutions / 解决方案:**

1. **sync.Mutex / 互斥锁**:
   ```go
   var mu sync.Mutex
   var data = make(map[string]int)
   func Set(k string, v int) {
       mu.Lock()
       data[k] = v
       mu.Unlock()
   }
   ```

2. **sync.RWMutex / 读写锁** (better for read-heavy):
   ```go
   var rwmu sync.RWMutex
   func Get(k string) int {
       rwmu.RLock()
       v := data[k]
       rwmu.RUnlock()
       return v
   }
   ```

3. **sync.Map / sync.Map** (best for infrequent writes):
   ```go
   var sm sync.Map
   sm.Store(key, value)
   if val, ok := sm.Load(key); ok { ... }
   ```

**Your Project / 你的项目:**
- web3-offchain: Plugin registry uses `sync.Map` (registered once, read frequently).
  web3-offchain中插件注册表使用sync.Map（一次性注册，频繁读取）。


## Part 2: Concurrency (并发编程)

### Q9: Worker Pool — 如何实现Worker Pool？

**Answer / 参考答案:**

```go
// Worker receives jobs from jobs channel, processes them,
// and sends results to results channel.
// Worker从jobs通道接收任务，处理后发送到results通道。

func worker(id int, jobs <-chan Job, results chan<- Result, wg *sync.WaitGroup) {
    defer wg.Done()
    for j := range jobs {
        result := process(j)
        results <- result
    }
}

func main() {
    jobs := make(chan Job, 100)
    results := make(chan Result, 100)

    // Start 3 workers
    // 启动3个worker
    var wg sync.WaitGroup
    for w := 1; w <= 3; w++ {
        wg.Add(1)
        go worker(w, jobs, results, &wg)
    }

    // Send 100 jobs
    // 发送100个任务
    for j := 1; j <= 100; j++ {
        jobs <- Job{ID: j}
    }
    close(jobs)

    // Wait for all workers to finish
    // 等待所有worker完成
    wg.Wait()
    close(results)

    // Collect results
    // 收集结果
    for r := range results {
        fmt.Printf("Result: %d\n", r)
    }
}
```

**Key Points / 关键点:**
- Buffered channels prevent goroutine deadlock.
  缓冲通道防止goroutine死锁。
- `wg.Wait()` ensures all workers finish before closing results.
  `wg.Wait()`确保所有worker完成后才关闭results通道。
- `defer wg.Done()` in worker ensures cleanup even on panic.
  worker中使用`defer wg.Done()`确保异常时也能清理。

---

### Q10: How does web3-offchain implement concurrent RPC polling?
**web3-offchain如何实现并发RPC轮询？**

**Answer / 参考答案:**

1. **Health Check / 健康检查**:
   - Concurrent health check using goroutines for all nodes.
     用goroutine并发探测所有节点。
   - Use channel to collect results, select the fastest healthy node.
     用channel收集结果，选最快的健康节点。

2. **Node Pool Protection / 节点池保护**:
   - RWMutex protects the node pool (frequent reads, rare updates).
     RWMutex保护节点池（频繁读，偶尔更新）。

3. **Auto Failover / 自动故障转移**:
   - When a node fails, automatically switch to backup.
     节点失败时自动切换到备用节点。
   - Context controls timeout for each RPC call.
     Context控制每个RPC调用的超时。

4. **Event Parsing / 事件解析**:
   - Worker pool pattern for concurrent event parsing.
     用Worker Pool模式并发解析事件。
   - Redis distributed lock prevents duplicate consumption.
     Redis分布式锁防止重复消费。

---

### Q11: Distributed Lock — 分布式锁怎么实现？

**Answer / 参考答案:**

**Redis Distributed Lock / Redis分布式锁:**

1. **Acquire Lock / 加锁**:
   ```
   SET key value NX PX timeout
   ```
   - Atomic operation: only one client can acquire the lock.
     原子操作：只有一个客户端能获取锁。
   - NX: Only set if not exists.
     NX：不存在时才设置。
   - PX: Auto-expire after milliseconds (avoid deadlock).
     PX：毫秒级自动过期（防止死锁）。

2. **Release Lock / 释放锁**:
   - Use Lua script to check owner AND delete atomically.
     用Lua脚本原子性地检查owner并删除。
   - Never release another client's lock.
     绝不能释放其他客户端的锁。

3. **Your Project Implementation / 你的项目实现**:
   - Key: `lock:event_parser:{chain}:{block_height}`
   - Timeout: 30 seconds (business logic completes in ~10s).
   - Prevents multi-instance duplicate consumption.
     防止多实例重复消费。

**Alternative: Redlock Algorithm / 备选方案：Redlock算法**
- More complex, requires multiple Redis instances.
  更复杂，需要多个Redis实例。
- Usually unnecessary for most use cases.
  大多数场景不需要。

---

### Q12: Graceful Shutdown — 优雅关闭怎么实现？

**Answer / 参考答案:**

1. **Signal Handling / 信号处理**:
   ```go
   sigCh := make(chan os.Signal, 1)
   signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
   <-sigCh  // Block until signal received
   ```

2. **Context Cancellation / 取消上下文**:
   - Create context with cancel function.
     创建带cancel函数的context。
   - Call cancel() when signal received.
     收到信号时调用cancel()。

3. **Drain Active Requests / 排空活跃请求**:
   - Stop accepting new requests.
     停止接收新请求。
   - Wait for in-flight requests to complete.
     等待进行中的请求完成。
   - Use `sync.WaitGroup` to track active goroutines.
     用sync.WaitGroup追踪活跃的goroutine。

4. **Close Resources / 关闭资源**:
   - Close DB (Database) connection pool.
     关闭数据库连接池。
   - Close Redis connection.
     关闭Redis连接。
   - Close RPC clients.
     关闭RPC客户端。

**Your Project / 你的项目:**
- web3-offchain: Signal handler → context.cancel → all workers exit → close DB/Redis/RPC.
  web3-offchain：信号处理 → context.cancel → 所有worker退出 → 关闭DB/Redis/RPC。

---

### Q13: Race Condition — 如何检测和处理竞态条件？

**Answer / 参考答案:**

1. **Detection / 检测**:
   - Run with `-race` flag: `go run -race main.go`
     使用-race标志运行：`go run -race main.go`
   - CI/CD (Continuous Integration/Continuous Deployment) pipeline should include race detection.
     CI/CD（持续集成/持续部署）流水线应包含竞态检测。

2. **Common Causes / 常见原因**:
   - Multiple goroutines writing to the same variable without synchronization.
     多个goroutine无同步地写入同一变量。
   - Channel misuse (sending on closed channel, etc.).
     通道误用（向已关闭的通道发送数据等）。

3. **Fixes / 修复方法**:
   - Use mutex for shared state.
     对共享状态使用mutex。
   - Use channels for message passing (don't share memory by communicating, communicate by sharing memory).
     用通道传递消息（不要通过共享内存来通信，而要通过通信来共享内存）。
   - Use `atomic` package for simple counters.
     对简单计数器使用atomic包。

4. **Your Project / 你的项目**:
   - web3-offchain: Each worker has its own goroutine, no shared mutable state except through channels.
     web3-offchain：每个worker有自己的goroutine，除通道外无共享可变状态。


## Part 3: Database (PostgreSQL)

### Q14: PostgreSQL Index Types — PostgreSQL索引类型有哪些？

**Answer / 参考答案:**

| Type / 类型 | Use Case / 适用场景 | Example / 示例 |
|-------------|---------------------|----------------|
| **B-Tree** (default) | Equality, range queries | `WHERE status = 1`, `WHERE block_number > 1000` |
| **Hash** | Equality only, fast lookup | `WHERE tx_hash = 'abc...'` |
| **GiST** | Full-text search, geometric types | `WHERE content @@ 'query'` |
| **GIN** (Go INternal) | Arrays, JSONB, composite types | `WHERE event_data @> '{"key":"value"}'` |
| **BRIN** | Large tables with sequential data | `WHERE block_number BETWEEN 1000 AND 2000` |

**Your Project Application / 你的项目中的应用:**
- web3-offchain: B-Tree index on `tx_hash` for fast lookups.
  web3-offchain中对tx_hash使用B-Tree索引加速查询。
- GIN index on `event_data` (JSONB) for flexible querying.
  对event_data（JSONB）使用GIN索引支持灵活查询。
- BRIN index on `block_number` (sequential writes).
  对block_number使用BRIN索引（按顺序写入）。

---

### Q15: How to Optimize Slow Queries? — 怎么优化慢查询？

**Answer / 参考答案:**

1. **EXPLAIN ANALYZE / 查看执行计划**:
   ```sql
   EXPLAIN ANALYZE SELECT * FROM events WHERE chain = 'eth' ORDER BY block_number DESC LIMIT 100;
   ```
   - Look for `Seq Scan` vs `Index Scan`.
     查看是顺序扫描还是索引扫描。
   - Check actual rows vs estimated rows.
     检查实际行数与预估行数的差异。

2. **Check Index Usage / 检查索引使用情况**:
   - Missing indexes on WHERE/JOIN columns.
     WHERE和JOIN列缺少索引。
   - Over-indexing (indexes slow down writes).
     索引过多（会降低写入速度）。

3. **Optimize JOINs / 优化JOIN**:
   - Join on indexed columns.
     在索引列上JOIN。
   - Avoid N+1 query problems.
     避免N+1查询问题。

4. **Select Only Needed Columns / 只查需要的字段**:
   - Don't use `SELECT *`.
     不要用SELECT *。

5. **Partitioning / 分区表**:
   - Partition by `block_number` for large tables.
     对大表按block_number分区。
   - Improves query performance for time-range scans.
     提升时间范围扫描的性能。

6. **Connection Pooling / 连接池**:
   - Use `pgx` or `lib/pq` with proper pool settings.
     使用pgx或lib/pq并配置合适的连接池。
   - `MaxOpenConns`, `MaxIdleConns`, `ConnMaxLifetime`.
     最大连接数、最大空闲连接数、连接最大生命周期。

**Your Project / 你的项目:**
- web3-offchain: Used EXPLAIN ANALYZE to optimize batch inserts.
  web3-offchain中使用EXPLAIN ANALYZE优化批量插入。
- Consider partitioning for historical event data.
  考虑对历史事件数据使用分区表。

---

### Q16: Transaction Isolation Levels — 事务隔离级别有哪些？

**Answer / 参考答案:**

| Level / 级别 | Dirty Read / 脏读 | Non-repeatable Read / 不可重复读 | Phantom Read / 幻读 |
|--------------|-------------------|----------------------------------|---------------------|
| Read Uncommitted | Possible / 可能 | Possible / 可能 | Possible / 可能 |
| Read Committed (default in PostgreSQL) | No / 否 | Possible / 可能 | Possible / 可能 |
| Repeatable Read | No / 否 | No / 否 | Possible / 可能 |
| Serializable | No / 否 | No / 否 | No / 否 |

**PostgreSQL Default: Read Committed.**
**PostgreSQL默认：Read Committed。**

**Your Project / 你的项目:**
- web3-offchain: Uses Read Committed for event ingestion (no strict consistency needed).
  web3-offchain使用Read Committed处理事件摄入（不需要严格一致性）。
- AEP contract interactions: Uses Serializable for fund transfers.
  AEP合约交互使用Serializable处理资金转账。

---

### Q17: Batch Insert Optimization — 批量插入如何优化？

**Answer / 参考答案:**

1. **Use INSERT ... VALUES (...), (...), (...) / 使用多行VALUES**:
   ```sql
   INSERT INTO events (chain, block_number, tx_hash, log_index) VALUES
     ('eth', 1000, 'abc...', 0),
     ('eth', 1000, 'def...', 1),
     ('bsc', 2000, 'ghi...', 0);
   ```

2. **Transaction Wrapping / 事务包裹**:
   - Wrap multiple inserts in one transaction.
     将多次插入包裹在一个事务中。
   - Reduces COMMIT overhead.
     减少COMMIT开销。

3. **Batch Size / 批次大小**:
   - Too small: High COMMIT overhead.
     太小：COMMIT开销大。
   - Too large: Memory pressure, longer transactions.
     太大：内存压力，事务时间过长。
   - Sweet spot: 500-2000 rows per batch.
     最佳范围：每批500-2000行。

4. **Your Project / 你的项目:**
   - web3-offchain: Batch size of 1000 rows, wrapped in transactions.
     web3-offchain：每批1000行，包裹在事务中。

---

## Part 4: Redis

### Q18: Redis Persistence — Redis持久化方式有哪些？

**Answer / 参考答案:**

1. **RDB (Snapshot / 快照)**:
   - Saves data snapshot at intervals.
     定期保存数据快照。
   - Fast recovery, but may lose data since last snapshot.
     恢复快，但可能丢失最后一次快照后的数据。
   - Suitable for backups.
     适合备份场景。
   - Config: `save 900 1` (snapshot if 1 key changed in 900s).
     配置：save 900 1（900秒内有1个key变化就快照）。

2. **AOF (Append-Only File / 追加日志)**:
   - Records every write command.
     记录每条写命令。
   - Safer data, but larger file and slower recovery.
     数据更安全，但文件大恢复慢。
   - Suitable for data-first scenarios.
     适合数据安全第一的场景。

**Your Project / 你的项目:**
- web3-offchain uses RDB (cache only, no persistence needed).
  web3-offchain使用RDB（只需要缓存，不需要持久化）。

---

### Q19: Cache Penetration/Breakdown/Avalanche — 缓存穿透/击穿/雪崩？

**Answer / 参考答案:**

| Issue / 问题 | Description / 描述 | Solution / 解决方案 |
|--------------|-------------------|---------------------|
| **Penetration / 穿透** | Query non-existent data, bypassing cache | Bloom filter, cache null values |
| **Breakdown / 击穿** | Hot key expires, massive requests hit DB | Mutex lock, never-expire keys |
| **Avalanche / 雪崩** | Many keys expire simultaneously | Add random TTL jitter |

**Your Project / 你的项目:**
- web3-offchain: Bloom filter prevents invalid contract address queries.
  web3-offchain中用布隆过滤器防止无效合约地址查询。
- Random TTL prevents avalanche.
  用随机过期时间防止雪崩。

---

### Q20: Redis Data Structures — Redis常用数据结构？

**Answer / 参考答案:**

| Structure / 结构 | Use Case / 适用场景 | Commands / 命令 |
|-----------------|---------------------|-----------------|
| **String** | Counter, cache, lock | SET, GET, INCR, SETNX |
| **Hash** | Object storage | HSET, HGET, HMGET |
| **List** | Queue, timeline | LPUSH, RPUSH, LPOP |
| **Set** | Unique collection, dedup | SADD, SMEMBERS, SISMEMBER |
| **Sorted Set** | Leaderboard, ranked data | ZADD, ZRANGE, ZRANK |

**Your Project / 你的项目:**
- web3-offchain: String for RPC node health status.
  web3-offchain中用String存储RPC节点健康状态。
- Hash for plugin configuration cache.
  用Hash缓存插件配置。
- Sorted Set for block number ranking (reorg detection).
  用Sorted Set存储区块高度排名（检测reorg）。

---

### Q21: Redis Distributed Lock Defects — Redis分布式锁的缺陷？

**Answer / 参考答案:**

1. **Clock Skew / 时钟漂移**:
   - If server clock jumps, lock may expire prematurely.
     如果服务器时钟跳跃，锁可能过早过期。

2. **Network Partition / 网络分区**:
   - Master receives lock, fails to replicate to slave before crash.
     主节点收到锁，但在复制到从节点前崩溃。
   - New master doesn't have the lock → duplicate acquisition.
     新主节点没有锁 → 重复获取。

3. **Solution: Redlock Algorithm / 解决方案：Redlock算法**:
   - Acquire lock on majority of N Redis instances.
     在N个Redis实例中的多数实例上获取锁。
   - More complex, but more reliable.
     更复杂，但更可靠。

4. **Alternative: etcd/ZooKeeper / 备选方案**:
   - Strong consistency guarantees (CP systems).
     强一致性保证（CP系统）。
   - Better for critical distributed locks.
     更适合关键分布式锁场景。


## Part 5: Blockchain / Web3 (区块链/Web3)

### Q22: Ethereum Transaction Flow — 以太坊交易流程是怎样的？

**Answer / 参考答案:**

1. **User Signs Transaction / 用户签名交易**:
   - Private key signs the transaction data.
     私钥对交易数据进行签名。

2. **Broadcast to P2P Network / 广播到P2P网络**:
   - Transaction enters the mempool (pending pool).
     交易进入mempool（待处理池）。

3. **Miner/Validator Includes in Block / 矿工/验证者打包**:
   - Miner selects transactions by gas price.
     矿工按gas价格选择交易。
   - Transaction is included in a block.
     交易被包含在区块中。

4. **Confirmation / 确认**:
   - Each new block adds one confirmation.
     每个新区块增加一个confirmation。
   - 6 confirmations = generally considered safe.
     6个confirmation通常被认为安全。

**Your Project / 你的项目:**
- web3-offchain listens to transaction events (doesn't send transactions).
  web3-offchain监听交易事件（不发交易）。
- Handles reorg: deletes events from reorganized blocks.
  处理reorg：删除回滚区块的事件。

---

### Q23: RPC Nodes — 什么是RPC节点？怎么选择？

**Answer / 参考答案:**

**What is RPC / RPC是什么:**
An RPC (Remote Procedure Call) node provides Ethereum API (Application Programming Interface) endpoints via HTTP (HyperText Transfer Protocol)/WebSocket.
RPC (远程过程调用) 节点通过HTTP (超文本传输协议)/WebSocket提供以太坊API端点。

**Popular Providers / 常用提供商:**

| Provider / 提供商 | Free Tier / 免费额度 | Paid / 付费 | Notes / 备注 |
|-------------------|---------------------|-------------|-------------|
| Infura | Limited | Per-request | OG provider |
| Alchemy | Generous | Per-request | Good performance |
| QuickNode | Limited | Per-request | Low latency |
| Self-hosted | N/A | Hardware cost | Full control |

**Your Project / 你的项目:**
- web3-offchain: Multi-node polling with automatic failover.
  web3-offchain：多节点轮询，自动故障转移。
- WebSocket for subscribing to new blocks (more efficient than HTTP polling).
  用WebSocket订阅新区块（比HTTP轮询更高效）。

---

### Q24: ERC-20 vs ERC-721 — ERC-20和ERC-721的区别？

**Answer / 参考答案:**

| Feature / 特性 | ERC-20 (Fungible Token / 同质化代币) | ERC-721 (NFT / Non-Fungible Token / 非同质化代币) |
|----------------|---------------------------------------|-------------------------------|
| Interchangeability / 可互换 | Yes, each token is identical | No, each token is unique |
| Decimals / 小数位 | Yes (usually 18) | No |
| Transfer / 转账 | `transfer(to, amount)` | `transferFrom(from, to, tokenId)` |
| Balance / 余额 | `balanceOf(address)` | `ownerOf(tokenId)` |
| Example / 示例 | USDT, LINK | CryptoKitties, Bored Apes |

**Your Project / 你的项目:**
- AEP handles ERC-20 transfer events.
  AEP中处理ERC-20代币的转账事件。
- Contract interaction uses abi.json to parse events.
  合约交互用abi.json解析event。

---

### Q25: What is CAW (Cobo Agentic Wallet)? — CAW是什么？

**Answer / 参考答案:**

CAW is Cobo's Agent Wallet standard with these features:
CAW是Cobo推出的Agent钱包标准，特点：

1. **Multi-signature / 多签**: Supports M-of-N signatures.
   支持M-of-N多签。

2. **Policy Rules / 策略规则**: Time-based limits, whitelist, amount caps.
   定时、限额、白名单等策略规则。

3. **On-chain + Off-chain / 链上链下结合**: Policies enforced on-chain.
   策略在链上执行。

4. **AI Agent Friendly / 适合AI Agent**: Automated transactions with safety controls.
   自动化交易+安全控制，适合AI Agent使用。

**Your Project / 你的项目:**
- AEP integrates CAW for fund custody.
  AEP中集成CAW实现资金托管。
- Uses CAW Pact to lock funds, releases after task completion.
  用CAW Pact锁定资金，Agent完成任务后释放。

---

### Q26: What is ERC-8183? — ERC-8183是什么？

**Answer / 参考答案:**

ERC-8183 is Cobo's standard for Agent Wallets (CAW). It defines:
ERC-8183是Cobo的Agent钱包标准（CAW），定义了：

1. **Agent Registration / 代理注册**: How agents register with the wallet.
   代理如何向钱包注册。

2. **Policy Enforcement / 策略执行**: On-chain policy rules.
   链上策略规则。

3. **Transaction Signing / 交易签名**: Multi-sig workflow for agents.
   代理的多签工作流程。

4. **Audit Trail / 审计追踪**: All agent actions are recorded.
   所有代理操作都有记录。

**Note / 注意:**
A judge at the Cobo hackathon noted my AEP project is essentially a hardcore implementation of ERC-8183.
Cobo黑客松评委指出我的AEP项目本质上是ERC-8183的硬核实现。

---

### Q27: How to Handle Blockchain Reorg? — 如何处理区块链reorg（区块回滚）？

**Answer / 参考答案:**

1. **Detect Reorg / 检测reorg**:
   - When a new block's parent_hash doesn't match the expected hash.
     当新区块的parent_hash不匹配时。

2. **Rollback / 回滚**:
   - Delete events from the reorganized blocks.
     删除回滚区块的事件。

3. **Replay / 重放**:
   - Resync from the last common ancestor block.
     从最后一个共同祖先区块重新同步。

4. **Idempotency / 幂等性**:
   - Ensure replay doesn't cause duplicate data.
     确保重放不会导致数据重复。

**Your Project Implementation / 你的项目实现:**
- web3-offchain: Uses block_hash as unique index.
  web3-offchain用block_hash做唯一索引。
- On reorg: deletes events from affected blocks, then re-queries.
  reorg时先删除受影响区块的事件，然后重新查询。
- Uses transactions to ensure consistency.
  用事务保证一致性。


## Part 6: System Design (系统设计)

### Q28: Design a Blockchain Event Listener — 设计一个区块链事件监听器

**Answer / 参考答案:**

**Architecture / 架构:**

```
+-------------+     +--------------+     +-------------+
|  RPC Layer   |---->|  Parse Layer |---->|  Storage    |
|  (Multi-node |     |  (Plugin     |     |  (PostgreSQL|
|   failover)  |     |   architecture)|   |   + Redis)   |
+-------------+     +--------------+     +-------------+
```

1. **RPC Layer / RPC层**:
   - Poll multiple RPC nodes per chain.
     每链轮询多个RPC节点。
   - Auto-switch on failure (health check).
     故障时自动切换（健康检查）。

2. **Parse Layer / 解析层**:
   - Plugin architecture: each contract type has its own parser.
     插件化架构：每种合约类型有自己的解析器。
   - Concurrent processing using worker pool.
     用worker pool并发处理。

3. **Storage Layer / 存储层**:
   - Batch insert to PostgreSQL (1000 rows/batch).
     批量插入PostgreSQL（1000行/批）。
   - Redis for caching and distributed locking.
     Redis用于缓存和分布式锁。

4. **Scheduler / 调度层**:
   - Process blocks in order by block number.
     按区块高度顺序处理。
   - Handle reorg by deleting and replaying.
     通过删除并重放处理reorg。

**Your Project / 你的项目:**
- web3-offchain implements this exact architecture.
  web3-offchain实现了上述完整架构。

---

### Q29: How to Handle High-Throughput Event Processing?
**如何处理高吞吐量的事件处理？**

**Answer / 参考答案:**

1. **Parallel Parsing / 并行解析**:
   - Use goroutine pool for event parsing.
     用goroutine池解析事件。
   - Each goroutine handles events from different contracts.
     每个goroutine处理不同合约的事件。

2. **Batch Writing / 批量写入**:
   - Buffer events in memory, flush in batches.
     在内存中缓冲事件，批量刷新。
   - Optimal batch size: 500-2000 rows.
     最佳批次大小：500-2000行。

3. **Connection Pooling / 连接池**:
   - Use pgx connection pool with proper settings.
     使用pgx连接池并配置合理参数。
   - `MaxOpenConns = runtime.NumCPU() * 2`.
     最大连接数 = CPU核心数 * 2。

4. **Index Optimization / 索引优化**:
   - Only index columns used in WHERE/JOIN.
     只为WHERE和JOIN使用的列建索引。
   - Use partial indexes for filtered data.
     对过滤数据使用部分索引。

5. **Monitoring / 监控**:
   - Track events processed per second.
     跟踪每秒处理的事件数。
   - Alert on lag (blocks behind).
     滞后时告警（区块落后）。

**Performance Metrics / 性能指标:**
- Throughput: ~500 events/sec.
  吞吐量：约500事件/秒。
- Latency: 3-5 seconds from block to storage.
  延迟：从出块到入库约3-5秒。
- Resource: 2 CPU (Central Processing Unit) cores, 512MB RAM (Random Access Memory).
  资源：2核CPU（中央处理器），512MB RAM（随机存取存储器）。

---

### Q30: Microservices Communication — 微服务间如何通信？

**Answer / 参考答案:**

| Method / 方式 | Synchronous / 同步 | Asynchronous / 异步 | Use Case / 适用场景 |
|---------------|-------------------|---------------------|---------------------|
| **REST** (Representational State Transfer)/HTTP | Yes / 是 | No / 否 | Simple request-response |
| **gRPC** | Yes / 是 | Yes / 是 (streaming) | High-performance internal comms |
| **Message Queue** | No / 否 | Yes / 是 | Decoupled services, event-driven |
| **Shared DB** | Both / 两者 | Both / 两者 | Simple architectures (not recommended) |

**Your Project / 你的项目:**
- web3-offchain: Single service, no microservices needed.
  web3-offchain：单体服务，不需要微服务。
- AEP: Backend communicates with frontend via REST + SSE.
  AEP：后端通过REST+SSE与前端的通信。

---

### Q31: API Design — RESTful API设计规范？

**Answer / 参考答案:**

1. **Resource Naming / 资源命名**:
   - Use nouns, plural: `/events`, `/contracts`, `/chains`.
     使用名词复数：/events、/contracts、/chains。

2. **HTTP Methods / HTTP方法**:
   - GET: Retrieve resource.
     获取资源。
   - POST: Create resource.
     创建资源。
   - PUT: Update entire resource.
     更新整个资源。
   - PATCH: Update partial resource.
     部分更新资源。
   - DELETE: Remove resource.
     删除资源。

3. **Status Codes / 状态码**:
   - 200 OK, 201 Created, 204 No Content.
   - 400 Bad Request, 401 Unauthorized, 403 Forbidden.
   - 404 Not Found, 409 Conflict.
   - 500 Internal Server Error, 502 Bad Gateway.

4. **Pagination / 分页**:
   - Use cursor-based pagination for large datasets.
     对大数据集使用游标分页。
   - Response format: `{ data: [...], next_cursor: "...", has_more: true }`.

5. **Error Response / 错误响应**:
   ```json
   {
     "error": {
       "code": "EVENT_NOT_FOUND",
       "message": "Event with hash abc... not found",
       "details": {}
     }
   }
   ```

---

## Part 7: Project Deep Dive (项目深挖)

### Q32: Why Go instead of Java? — 为什么选择Go而不是Java？

**Answer / 参考答案:**

1. **Concurrency Model / 并发模型**:
   - Goroutines are lighter than threads.
     goroutine比线程更轻量。
   - Simpler concurrency with channels.
     用channel实现更简单的并发。

2. **Deployment / 部署**:
   - Static binary, no JVM needed.
     静态二进制文件，不需要JVM。
   - Smaller Docker images (20MB vs 300MB+).
     Docker镜像更小（20MB vs 300MB+）。

3. **Blockchain Ecosystem / 区块链生态**:
   - Go is the mainstream language for Web3 (Geth, Tendermint, etc.).
     Go是Web3的主流语言（Geth、Tendermint等）。

4. **Performance / 性能**:
   - Close to C, but with higher developer productivity.
     性能接近C，但开发效率更高。

5. **My Decision / 我的选择**:
   - For web3-offchain: Go's goroutine pool is ideal for concurrent RPC calls.
     对于web3-offchain：Go的goroutine池非常适合并发RPC调用。
   - I kept my Java foundation — if a project needs Spring ecosystem, I can use Java.
     我保留了Java基础——如果项目需要Spring生态，我也可以用Java。

---

### Q33: What are the performance metrics of web3-offchain?
**web3-offchain的性能指标是多少？**

**Answer / 参考答案:**

| Metric / 指标 | Value / 数值 |
|---------------|-------------|
| Throughput / 吞吐量 | ~500 events/sec |
| Latency / 延迟 | 3-5 seconds (block to storage) |
| CPU / CPU | 2 cores |
| Memory / 内存 | 512MB |
| Batch Size / 批次大小 | 1000 rows/batch |
| Concurrent Workers / 并发worker | 3 goroutines |

**Optimization Methods / 优化手段:**
- Batch writes reduce DB connection overhead.
  批量写入减少DB连接开销。
- Index optimization accelerates queries.
  索引优化加速查询。
- Redis caching for hot data.
  Redis缓存热点数据。

---

### Q34: If you could refactor web3-offchain, what would you change?
**如果让你重构web3-offchain，你会改什么？**

**Answer / 参考答案:**

1. **Add Monitoring / 增加监控**:
   - Prometheus + Grafana for metrics visualization.
     Prometheus+Grafana可视化监控指标。

2. **Add Tests / 增加测试**:
   - Unit test coverage for plugins and storage layer.
     对插件和存储层增加单元测试覆盖。

3. **Add CI/CD / 增加CI/CD**:
   - GitHub Actions for automated build and deploy.
     GitHub Actions自动化构建和部署。

4. **Improve Error Handling / 优化错误处理**:
   - Unified error handling with custom error types.
     自定义错误类型的统一错误处理。

5. **Add Documentation / 增加文档**:
   - API documentation with Swagger/OpenAPI.
     用Swagger/OpenAPI编写API文档。

**But / 但是:**
These are "nice-to-haves." The current version is sufficient to demonstrate capability.
这些都是"锦上添花"，当前版本已经足够证明能力。

---

### Q35: How do you explain your 15-month gap? — 怎么解释15个月的空窗期？

**Answer / 参考答案 (Chinese / 中文版):**

"这段时间我全职在做个人项目和技术转型。4年Java后端之后，我意识到Web3和Go是方向，所以花了15个月系统学习并完成两个完整项目：
- web3-offchain：EVM链上数据索引器，Go + Gin + GORM + PostgreSQL + Redis
- AEP：AI代理协作平台，Go + Solidity + React

现在我觉得能力已经足够，想回到正式团队。"

**Answer / 参考答案 (English / 英文版):**

"I've been fully focused on personal projects and technology transition. After 4 years in Java backend, I realized Web3 and Go are the future, so I spent 15 months systematically learning and completing two full projects:
- web3-offchain: EVM (Ethereum Virtual Machine) event listener with Go, Gin, GORM, PostgreSQL, Redis
- AEP: AI agent collaboration platform with Go, Solidity, React

Now I feel my skills are solid enough and I'm ready to join a professional team.

---

### Q36: How do you respond to "Was this project AI-generated?" — 被问"项目是AI写的吗？"怎么回答？

**Answer / 参考答案 (Chinese / 中文版):**

"我用AI辅助开发，但核心架构设计和关键代码是我自己写的。AI帮我生成了一些样板代码和测试用例，但架构决策、性能优化、错误处理都是我自己完成的。我的项目已经部署上线，代码在GitHub上可以查看。"

**Answer / 参考答案 (English / 英文版):**

"I use AI as a development assistant, but the core architecture design and critical code are my own. AI helped generate some boilerplate code and test cases, but architectural decisions, performance optimization, and error handling are all done by me. My projects are deployed and the code is available on GitHub for review."


## Part 8: Algorithm Questions (算法题)

### Q37: Two Sum — 两数之和 (Easy)

**Problem / 题目:** Given an array of integers `nums` and an integer `target`, return indices of the two numbers such that they add up to `target`.

给定整数数组 `nums` 和目标值 `target`，返回两数之和的索引。

```go
// O(n) time, O(n) space
// 时间复杂度O(n)，空间复杂度O(n)
func twoSum(nums []int, target int) []int {
    seen := make(map[int]int) // value -> index
    for i, num := range nums {
        complement := target - num
        if idx, ok := seen[complement]; ok {
            return []int{idx, i}
        }
        seen[num] = i
    }
    return nil
}
```

---

### Q38: Reverse Linked List — 反转链表 (Easy)

**Problem / 题目:** Given the head of a singly linked list, reverse the list and return the reversed list.

给定单链表头节点，反转链表并返回。

```go
// O(n) time, O(1) space
// 时间复杂度O(n)，空间复杂度O(1)
func reverseList(head *ListNode) *ListNode {
    var prev *ListNode
    curr := head
    for curr != nil {
        next := curr.Next // Save next node
        curr.Next = prev  // Reverse pointer
        prev = curr       // Move prev forward
        curr = next       // Move curr forward
    }
    return prev
}
```

---

### Q39: Longest Substring Without Repeating Characters — 最长无重复字符子串 (Medium)

**Problem / 题目:** Given a string `s`, find the length of the longest substring without repeating characters.

给定字符串 `s`，找出不含重复字符的最长子串长度。

```go
// O(n) time, O(min(m,n)) space where m is charset size
// 时间复杂度O(n)，空间复杂度O(min(m,n))
func lengthOfLongestSubstring(s string) int {
    seen := make(map[byte]int) // char -> last index
    left, maxLen := 0, 0
    for right := 0; right < len(s); right++ {
        // If char seen and within current window, move left pointer
        if idx, ok := seen[s[right]]; ok && idx >= left {
            left = idx + 1
        }
        seen[s[right]] = right
        if right-left+1 > maxLen {
            maxLen = right - left + 1
        }
    }
    return maxLen
}
```

---

### Q40: Valid Parentheses — 有效括号 (Easy)

**Problem / 题目:** Given a string containing just '(', ')', '{', '}', '[' and ']', determine if the input string is valid.

仅含'(){}[]'的字符串，判断括号是否合法匹配。

```go
// O(n) time, O(n) space
// 时间复杂度O(n)，空间复杂度O(n)
func isValid(s string) bool {
    stack := []rune{}
    pairs := map[rune]rune{
        ')': '(', '}': '{', ']': '[',
    }
    for _, c := range s {
        if open, ok := pairs[c]; ok {
            // Closing bracket
            // 右括号
            if len(stack) == 0 || stack[len(stack)-1] != open {
                return false
            }
            stack = stack[:len(stack)-1] // Pop
        } else {
            // Opening bracket
            // 左括号
            stack = append(stack, c)
        }
    }
    return len(stack) == 0
}
```

---

### Q41: Merge Two Sorted Lists — 合并两个有序链表 (Easy)

**Problem / 题目:** Merge two sorted linked lists and return it as a sorted list.

合并两个有序链表并返回新的有序链表。

```go
// O(n+m) time, O(1) space (iterative)
// 时间复杂度O(n+m)，空间复杂度O(1)（迭代法）
func mergeTwoLists(l1, l2 *ListNode) *ListNode {
    dummy := &ListNode{}
    curr := dummy
    for l1 != nil && l2 != nil {
        if l1.Val < l2.Val {
            curr.Next = l1
            l1 = l1.Next
        } else {
            curr.Next = l2
            l2 = l2.Next
        }
        curr = curr.Next
    }
    if l1 != nil {
        curr.Next = l1
    }
    if l2 != nil {
        curr.Next = l2
    }
    return dummy.Next
}
```

---

### Q42: Maximum Subarray — 最大子数组和 (Medium)

**Problem / 题目:** Find the contiguous subarray with the largest sum.

找出和最大的连续子数组。

```go
// Kadane's algorithm: O(n) time, O(1) space
// 动态规划：时间复杂度O(n)，空间复杂度O(1)
func maxSubArray(nums []int) int {
    maxSum := nums[0]
    currentSum := nums[0]
    for i := 1; i < len(nums); i++ {
        // Either extend previous subarray or start new one
        if currentSum < 0 {
            currentSum = nums[i]
        } else {
            currentSum += nums[i]
        }
        if currentSum > maxSum {
            maxSum = currentSum
        }
    }
    return maxSum
}
```

---

### Q43: LRU Cache — LRU缓存 (Medium)

**Problem / 题目:** Design and implement an LRU (Least Recently Used) cache.

设计和实现LRU（最近最少使用）缓存。

```go
// Using doubly linked list + hash map
// 双向链表 + 哈希表实现
type LRUCache struct {
    capacity int
    cache    map[int]*Node
    head     *Node // Dummy head
    tail     *Node // Dummy tail
}

type Node struct {
    key, val int
    prev, next *Node
}

func (n *Node) remove() {
    n.prev.next = n.next
    n.next.prev = n.prev
}

func (n *Node) pushFront(head *Node) {
    n.prev = head
    n.next = head.next
    head.next.prev = n
    head.next = n
}

func Constructor(capacity int) LRUCache {
    head := &Node{}
    tail := &Node{}
    head.next = tail
    tail.prev = head
    return LRUCache{
        capacity: capacity,
        cache:    make(map[int]*Node),
        head:     head,
        tail:     tail,
    }
}

func (c *LRUCache) Get(key int) int {
    if node, ok := c.cache[key]; ok {
        node.remove()       // Move to front
        node.pushFront(c.head)
        return node.val
    }
    return -1
}

func (c *LRUCache) Put(key int, value int) {
    if node, ok := c.cache[key]; ok {
        node.val = value
        node.remove()
        node.pushFront(c.head)
    } else {
        if len(c.cache) >= c.capacity {
            // Evict least recently used (before tail)
            deleted := c.tail.prev
            deleted.remove()
            delete(c.cache, deleted.key)
        }
        newNode := &Node{key: key, val: value}
        newNode.pushFront(c.head)
        c.cache[key] = newNode
    }
}
```

---

## Part 9: English Interview Scripts (英语面试话术)

### Q44: Self Introduction (1 minute) — 一分钟自我介绍

```
Hi, I'm Zhuang Zhen. I have 4 years of Java backend experience,
and over the past 15 months, I've been fully focused on Go and
Web3 development.

I built two main projects: web3-offchain, an EVM event listener
that polls multiple chains via RPC with a plugin architecture,
batch writes to PostgreSQL, and Redis distributed locking. It's
deployed with Docker and supports graceful shutdown.

I also built AEP, an AI agent collaboration platform that
integrates CAW wallets, smart contracts, and a React frontend.
It was submitted to the Cobo Track hackathon, where a judge noted
my implementation aligns with ERC-8183 standard.

I'm looking for a remote Web3 backend position where I can apply
my Go and blockchain experience.
```

---

### Q45: Tell Me About Your Most Challenging Project
**讲讲你最有挑战性的项目**

```
My most challenging project is web3-offchain. The hardest part
was implementing concurrent RPC polling with automatic failover.

I needed to:
1. Health check multiple RPC nodes concurrently using goroutines
2. Select the fastest healthy node for each request
3. Automatically switch to backup nodes when one fails
4. Ensure no duplicate event processing across instances

I solved this by using a worker pool pattern with goroutines,
Redis distributed locks for consistency, and a RWMutex to
protect the node pool. The system can handle 500+ events per
second with less than 5 seconds latency.
```

---

### Q46: Why Do You Want to Work Remotely?
**你为什么想远程工作？**

```
I believe remote work increases productivity. I've been working
independently on my projects for 15 months, and I've learned to
manage my time effectively and communicate asynchronously.

Also, Web3 is inherently a global, remote-first industry. I want
to work in an environment that matches the nature of the
technology I'm building.
```

---

### Q47: How Do You Handle Disagreements in Code Reviews?
**代码评审中意见不合怎么处理？**

```
I listen first to understand their perspective. Then I explain
my reasoning with facts — performance data, benchmarks, or
documentation. If we still disagree, I propose a compromise or
let the team lead decide. The goal is the best code, not winning
the argument.
```

---

### Q48: What's Your Biggest Weakness?
**你最大的缺点是什么？**

```
My English speaking isn't perfect yet — I'm stronger in reading
and writing. That's why I've been practicing with AI conversations
and preparing interview scripts. I also haven't worked in a large
team environment recently, but I'm eager to collaborate again.
```

---

### Q49: Do You Have Any Questions for Us?
**你有什么想问我们的吗？**（面试结尾必问）

```
1. What does a typical day look like for this role?
   这个岗位典型的一天是什么样的？

2. What's the team structure and how does the engineering team
   collaborate?
   团队结构和工程团队如何协作？

3. What are the biggest technical challenges the team is facing
   right now?
   团队目前面临的最大技术挑战是什么？

4. What does success look like in the first 3 months?
   前3个月的成功标准是什么？

5. How does the company support professional development?
   公司如何支持员工职业发展？
```

---

### Q50: Common Meeting Phrases — 会议常用语

**Opening / 开场:**
- "Hi everyone, let me start by giving a quick update."
  大家好，我先快速汇报一下进展。
- "I'll cover the backend part first, then hand over."
  我先讲后端部分，然后交接。

**Can't Understand / 没听懂:**
- "Sorry, could you repeat that?"
  抱歉，能再说一遍吗？
- "Just to make sure I understood correctly, you're saying..."
  让我确认一下我理解的对不对，你是说……

**Expressing Opinion / 表达观点:**
- "From my perspective..."
  从我角度看……
- "I think the best approach is..."
  我认为最好的方法是……
- "The reason I chose this is..."
  我选择这个的原因是……

**Summarizing / 总结:**
- "To summarize, I've completed X and next I'll work on Y."
  总结一下，我已经完成了X，接下来要做Y。
- "Let me know if you have any questions."
  有问题随时问我。
