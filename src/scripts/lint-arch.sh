#!/usr/bin/env bash
# lint-arch —— 校验 v10 的三权边界 (见 docs/design/v10.md §5).
#
#   domain/ : 只定义接口/结构/类型/常量/全局变量 (零逻辑自由函数)
#   internal/: kits/<domain>kit 固定命名; 其余包名需语义明确; 承载可复用结构
#   engine/ : 每个子目录恰好一个具备完整生命周期的 engine
#
# 用法:
#   scripts/lint-arch.sh            # 在 src/ 下执行, 或在任意位置执行
#   task lint:arch                  # 同上 (Taskfile 封装)
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

FAIL=0
pass() { printf '  PASS  %s\n' "$1"; }
fail() { printf '  FAIL  %s\n' "$1"; FAIL=1; }
note() { printf '  NOTE  %s\n' "$1"; }
section() { printf '\n== %s ==\n' "$1"; }

# ---------------------------------------------------------------------------
section "1. domain 纯度"

# 1.1 domain 不得依赖 internal (R1: kits 依赖 domain, 反向禁止)
hits=$(grep -rn 'backend/internal/' backend/domain/ 2>/dev/null || true)
if [ -z "$hits" ]; then
    pass "domain 不依赖 internal"
else
    fail "domain 依赖了 internal (见下)"
    printf '%s\n' "$hits"
fi

# 1.2 domain 只允许声明: 自由函数仅 key.New 白名单 (R1 例外)
bad_funcs=$(grep -rn '^func [A-Z]' backend/domain/ 2>/dev/null \
    | grep -v 'func (' \
    | grep -v '^backend/domain/key/key.go:' || true)
if [ -z "$bad_funcs" ]; then
    pass "domain 无逻辑自由函数 (白名单: domain/key 的 New)"
else
    fail "domain 出现新的逻辑自由函数 (应下沉 internal/kits)"
    printf '%s\n' "$bad_funcs"
fi

# ---------------------------------------------------------------------------
section "2. internal 纯度"

# 2.1 目录名不得空泛
vague=$(find backend/internal -mindepth 1 -maxdepth 1 -type d -exec basename {} \; \
    | grep -xE 'tools|util|utils|common|misc|shared|helper|const|base|core' || true)
if [ -z "$vague" ]; then
    pass "internal 无空泛目录名"
else
    fail "internal 出现空泛目录名: $(printf '%s' "$vague" | tr '\n' ' ')"
fi

# 2.2 kits 命名固定 <domain>kit, 且 package 名与目录一致
kits_tmp=$(mktemp)
find backend/internal/kits -mindepth 1 -maxdepth 1 -type d -exec basename {} \; >"$kits_tmp" 2>/dev/null
kit_bad=0
while IFS= read -r kit; do
    [ -n "$kit" ] || continue
    case "$kit" in
    *kit) ;;
    *)
        fail "internal/kits/$kit 命名不符合 <domain>kit"
        kit_bad=1
        continue
        ;;
    esac
    pkg=$(grep -hm1 '^package ' backend/internal/kits/"$kit"/*.go | head -1 | sed 's/^package //')
    if [ "$pkg" != "$kit" ]; then
        fail "internal/kits/$kit 的 package 名为 '$pkg', 与目录不一致"
        kit_bad=1
    fi
    domain="${kit%kit}"
    if [ ! -d "backend/domain/$domain" ]; then
        note "kits/$kit 对应的 backend/domain/$domain 已不存在 (该 domain 只剩消费侧辅助)"
    fi
done <"$kits_tmp"
rm -f "$kits_tmp"
if [ "$kit_bad" -eq 0 ]; then
    pass "kits/* 命名与 package 名一致"
fi

# ---------------------------------------------------------------------------
section "3. engine 纯度"

# 3.1 engine/ 根目录不得直接放源码 (每个 engine 必须有自己的子目录)
root_go=$(find backend/engine -maxdepth 1 -name '*.go' 2>/dev/null || true)
if [ -z "$root_go" ]; then
    pass "engine/ 根目录无源码"
else
    fail "engine/ 根目录不应直接放 .go: $(printf '%s' "$root_go" | tr '\n' ' ')"
fi

# 3.2 每个含源码的 engine 子目录必须实现一个完整 Engine (Name/Deps/Init/Start/Stop)
eng_tmp=$(mktemp)
find backend/engine -mindepth 1 -type d >"$eng_tmp"
eng_bad=0
while IFS= read -r dir; do
    [ -n "$dir" ] || continue
    ls "$dir"/*.go >/dev/null 2>&1 || continue
    recv=$(grep -hoE 'func \(([a-zA-Z_]+) \*?([A-Za-z_]+)\) Name\(\) string' "$dir"/*.go 2>/dev/null \
        | sed -E 's/func \(([a-zA-Z_]+) \*?([A-Za-z_]+)\) Name.*/\1 \2/' | head -1)
    if [ -z "$recv" ]; then
        fail "$dir 未实现 Engine (缺少 Name() string)"
        eng_bad=1
        continue
    fi
    recv_name="${recv%% *}"
    type_name="${recv##* }"
    missing=""
    for m in Deps Init Start Stop; do
        grep -qE "func \(${recv_name} \*?${type_name}\) ${m}\(" "$dir"/*.go 2>/dev/null || missing="$missing $m"
    done
    if [ -n "$missing" ]; then
        fail "$dir 的 $type_name 缺少方法:$missing"
        eng_bad=1
    fi
done <"$eng_tmp"
rm -f "$eng_tmp"
if [ "$eng_bad" -eq 0 ]; then
    pass "每个 engine/*/ 恰好实现一个完整 Engine"
fi

# ---------------------------------------------------------------------------
section "4. 全局 (build / vet / gofmt)"

if out=$(go build . ./backend/... 2>&1); then
    pass "go build"
else
    fail "go build"
    printf '%s\n' "$out" | tail -20
fi

if out=$(go vet . ./backend/... 2>&1); then
    pass "go vet"
else
    fail "go vet"
    printf '%s\n' "$out" | tail -20
fi

unfmt=$(gofmt -l backend main.go 2>/dev/null || true)
if [ -z "$unfmt" ]; then
    pass "gofmt"
else
    fail "gofmt 未格式化:"
    printf '%s\n' "$unfmt"
fi

# ---------------------------------------------------------------------------
printf '\n'
if [ "$FAIL" -eq 0 ]; then
    printf 'arch-lint: OK\n'
else
    printf 'arch-lint: FAILED (见上方 FAIL 项)\n'
fi
exit "$FAIL"
