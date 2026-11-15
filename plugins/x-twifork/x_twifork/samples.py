"""--dry-run 内置样例推文。

用于在无真实 X Cookie、无核心服务的环境下生成完全合规的契约 payload。
内容为虚构演示数据（时间线形状与真实 twikit Tweet 一致：新→旧排列）。
"""

from __future__ import annotations

from .transform import TweetLike

SAMPLE_TWEETS: list[TweetLike] = [
    TweetLike(
        id="1900000000000000107",
        text=(
            "Breaking: our new small-model release drops next week — 3x cheaper inference, "
            "same quality class. Benchmarks + weights published at launch. "
            "https://example.com/blog/small-model"
        ),
        created_at_raw="Sun Nov 15 09:12:44 +0000 2026",
        screen_name="ai_builds_dev",
        user_name="AI Builds Dev",
        favorite_count=3421,
        retweet_count=812,
        reply_count=233,
        view_count=184000,
        urls=["https://example.com/blog/small-model"],
        lang="en",
    ),
    TweetLike(
        id="1900000000000000106",
        text=(
            "Leaked internal eval: agent framework X beats baseline by 12 pts on long-horizon tasks.\n"
            "\n"
            "Caveats: small sample size, single seed. Treat as signal, not proof."
        ),
        created_at_raw="Sun Nov 15 08:47:02 +0000 2026",
        screen_name="eval_watch",
        user_name="Eval Watch",
        favorite_count=1897,
        retweet_count=402,
        reply_count=156,
        view_count=96500,
        lang="en",
    ),
    TweetLike(
        id="1900000000000000105",
        text=(
            "RT @deal_radar_cn: 双11 返场：某云厂商 2C2G VPS 年付 30 元，库存不多，"
            "小号/插件部署节点可以冲 https://example.com/deal/vps"
        ),
        created_at_raw="Sun Nov 15 08:02:10 +0000 2026",
        screen_name="deal_radar_cn",
        user_name="Deal Radar",
        favorite_count=89,
        retweet_count=45,
        reply_count=12,
        view_count=12300,
        urls=["https://example.com/deal/vps"],
        lang="zh",
        is_retweet=True,
    ),
    TweetLike(
        id="1900000000000000104",
        text="早八人破防了，今天天气也不好，什么都不想干",
        created_at_raw="Sun Nov 15 07:31:55 +0000 2026",
        screen_name="random_noise_99",
        user_name="Random Noise",
        favorite_count=3,
        retweet_count=0,
        reply_count=2,
        view_count=210,
        lang="zh",
    ),
    TweetLike(
        id="1900000000000000103",
        text=(
            "Reminder: inference prices keep falling — rerun your cost model quarterly. "
            "Most 2024-era budgets are 5-10x stale."
        ),
        created_at_raw="Sun Nov 15 06:58:12 +0000 2026",
        screen_name="inference_econ",
        user_name="Inference Econ",
        favorite_count=517,
        retweet_count=98,
        reply_count=41,
        view_count=40200,
        lang="en",
    ),
    TweetLike(
        id="1900000000000000102",
        text=(
            "Thread: how we cut our monitoring bill 80% with a single SQLite table and a cron job "
            "(no Kafka, no ELK, no JVM)."
        ),
        created_at_raw="Sun Nov 15 06:20:40 +0000 2026",
        screen_name="infra_minimal",
        user_name="Infra Minimal",
        favorite_count=764,
        retweet_count=141,
        reply_count=30,
        view_count=61000,
        lang="en",
    ),
    TweetLike(
        id="1900000000000000101",
        text="Old benchmark thread from last month still getting replies — quoting for context.",
        created_at_raw="Sat Nov 14 15:11:09 +0000 2026",
        screen_name="eval_watch",
        user_name="Eval Watch",
        favorite_count=1203,
        retweet_count=250,
        reply_count=88,
        view_count=77000,
        quoted_status_id="1900000000000000090",
        lang="en",
    ),
]
