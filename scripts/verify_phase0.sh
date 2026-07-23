#!/bin/bash
set -e
cd "$(dirname "$0")/.."

echo "=== Phase 0 验证 ==="

# 1. Git 已初始化
test -d .git || { echo "❌ Git 未初始化"; exit 1; }
echo "✅ Git 已初始化"

# 2. .gitignore 包含 .env
grep -q "^\.env$" .gitignore || { echo "❌ .gitignore 缺少 .env"; exit 1; }
echo "✅ .gitignore 包含 .env"

# 3. CI 文件存在
test -f .github/workflows/ci.yml || { echo "❌ CI 配置缺失"; exit 1; }
echo "✅ CI 配置存在 (.github/workflows/ci.yml)"

# 4. foundry.toml 配置 evm_version = cancun（对齐 SD §2.3）
grep -q 'evm_version.*=.*"cancun"' contracts/foundry.toml || { echo "❌ foundry.toml 缺少 evm_version = \"cancun\""; exit 1; }
grep -q 'solc_version.*=.*"0.8.24"' contracts/foundry.toml || { echo "❌ foundry.toml 缺少 solc_version = \"0.8.24\""; exit 1; }
grep -q 'optimizer.*=.*true' contracts/foundry.toml || { echo "❌ foundry.toml 未开启 optimizer"; exit 1; }
echo "✅ foundry.toml 配置正确 (Cancun + 0.8.24 + optimizer)"

# 5. Forge build
echo "--- Forge build ---"
cd contracts
forge build
echo "✅ forge build 通过"

# 6. Forge test
echo "--- Forge test ---"
forge test --match-path test/PrismSettleRegistry.t.sol -vvv
echo "✅ forge test 通过"

cd ..
echo ""
echo "✅ Phase 0 验证通过（Foundry + Git + CI + Cancun EVM）"
