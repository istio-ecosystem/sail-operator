#!/usr/bin/env bash
# 步骤 1a：解析输入（流水线 run 或镜像地址）→ 确定待扫描镜像与基分支，初始化本次任务状态。
# 用法: resolve-input.sh <RUN_ID | run URL | 镜像地址>
#   run 模式:  只接受 "Alauda Release workflow" 的成功 run，从 "Output image: " step 名提取镜像
#   镜像模式:  直接使用给定镜像（必须是 servicemesh-operator2 主镜像，-bundle 不在范围）
# 基分支 = 当前检出分支（修复分支从它切出，PR 也以它为 base）。
# 退出码: 0=OK  1=失败
# 注意: run 模式需要 gh 认证；Bash timeout 建议 120000。

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

main() {
  repo_root
  [[ $# -eq 1 ]] || die "用法: resolve-input.sh <RUN_ID | run URL | 镜像地址>（只接受一个参数）"
  local ARG="$1"

  # ---------- 基分支 = 当前检出分支 ----------
  local BASE_BRANCH
  BASE_BRANCH="$(git branch --show-current)"
  [[ -n "$BASE_BRANCH" ]] || die "当前处于 detached HEAD，请先检出要作为修复基础的分支"

  # ---------- 残留上次任务状态时安全清理：worktree 有未提交改动则终止，交人工确认 ----------
  if [[ -f "$STATE_FILE" ]]; then
    local wt="$STATE_DIR/worktree"
    if [[ -d "$wt" ]]; then
      if [[ -n "$(git -C "$wt" status --porcelain 2>/dev/null)" ]]; then
        die "上次任务残留 worktree 有未提交改动: $wt
请人工确认（保留则提交，放弃则 git worktree remove --force）后重跑"
      fi
      info "清理上次任务的干净 worktree: $wt"
      git worktree remove --force "$wt" >/dev/null 2>&1 || true
    fi
    git worktree prune >/dev/null 2>&1 || true
    rm -rf "$STATE_DIR" && mkdir -p "$STATE_DIR"
  fi

  local IMG_TSV="$STATE_DIR/images-round1.tsv"
  : >"$IMG_TSV"

  # ---------- 输入解析 ----------
  local id="" MODE=""
  if [[ "$ARG" =~ ^[0-9]+$ ]]; then
    id="$ARG"; MODE=run
  elif [[ "$ARG" =~ /runs/([0-9]+) ]]; then
    id="${BASH_REMATCH[1]}"; MODE=run
  elif [[ "$ARG" == */*:* ]]; then
    MODE=image
  else
    die "无法识别参数（既不是 run ID/URL，也不是镜像地址）: $ARG"
  fi

  if [[ "$MODE" == run ]]; then
    require_gh
    local meta wf status conclusion head_branch url
    meta="$(gh run view "$id" --repo "$REPO" --json workflowName,status,conclusion,headBranch,url)" \
      || die "读取 run $id 失败（--repo $REPO）"
    wf="$(jq -r .workflowName <<<"$meta")"
    status="$(jq -r .status <<<"$meta")"
    conclusion="$(jq -r .conclusion <<<"$meta")"
    head_branch="$(jq -r .headBranch <<<"$meta")"
    url="$(jq -r .url <<<"$meta")"

    [[ "$wf" == "$WORKFLOW_NAME" ]] \
      || die "run $id 属于工作流「$wf」，本 skill 只处理「$WORKFLOW_NAME」"
    [[ "$status" == "completed" && "$conclusion" == "success" ]] \
      || die "run $id 未成功完成（status=$status conclusion=$conclusion），镜像可能不完整，不纳入扫描"

    info "提取 run $id 构建产出的镜像..."
    local all kept
    all="$(gh run view "$id" --repo "$REPO" --json jobs \
      --jq '.jobs[].steps[].name | select(startswith("Output image: ")) | sub("^Output image: "; "")')"
    kept="$(run_output_images "$id")"
    [[ -n "$kept" ]] || die "run $id 未产出扫描范围内的镜像（$IMAGE_REPO_NAME）。产出清单:
$all"

    while IFS= read -r img; do
      printf '1\trun:%s\t%s\n' "$id" "$img" >>"$IMG_TSV"
    done <<<"$kept"

    echo "run $id [$WORKFLOW_NAME] 分支 $head_branch"
    echo "  $url"
    echo "  全部产出镜像:"
    sed 's/^/    /' <<<"$all"
    echo "  纳入扫描（-bundle 镜像不会有漏洞，不在范围）:"
    sed 's/^/    /' <<<"$kept"
    if [[ "$head_branch" != "$BASE_BRANCH" ]]; then
      echo
      warn "run 的构建分支（$head_branch）与当前检出分支（$BASE_BRANCH）不一致。
      修复将基于当前分支 $BASE_BRANCH。若应基于 $head_branch，请先检出该分支再重跑本脚本"
    fi
  else
    in_scan_scope "$ARG" \
      || die "镜像 $ARG 不在扫描范围（只扫 $IMAGE_REPO_NAME 主镜像；-bundle 镜像不会有漏洞，无需扫描）"
    printf '1\targ\t%s\n' "$ARG" >>"$IMG_TSV"
    echo "输入镜像: $ARG"
  fi

  set_state ROUND 1
  set_state BASE_BRANCH "$BASE_BRANCH"

  echo
  echo "INPUT_RESOLVED"
  echo "BASE_BRANCH=$BASE_BRANCH"
  echo "下一步: 执行 scan-image.sh 扫描（Bash timeout 设 600000）"
}

main "$@"
