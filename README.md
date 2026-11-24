# mirage

一个用 Go + SQLite 写的个人技术博客站，有完整的权限体系。

- 公开页面对所有人统一
- 站长登录后多出管理工具条，能改内容、生成分享链接
- 分享链接可以把私密内容分享给指定的人，可设有效期、访问密码、访问次数
- 内容改动即时生效，没有构建环节
- 板块是数据不是代码：先做手记 / 关于 / 留言，以后想加「养鸟」「拍照」这类板块，后台插一行记录就行

这是个用来练手的项目。代码要读得懂、能学到东西，不是“跑起来就行”。每个阶段的学习收获记在 [docs/学习笔记.md](docs/学习笔记.md)。

## 新会话从这份开始

| 顺序 | 文档 | 讲什么 |
|---|---|---|
| **0** | **[docs/交接文档.md](docs/交接文档.md)** | **现在的状态、已经定死的、还没定的、下一步做什么。开工先读这份** |
| 1 | [docs/决策记录.md](docs/决策记录.md) | 为什么是现在这样：视觉返工、架构改写的理由、被推翻的决策 |
| 2 | [docs/项目说明文档.md](docs/项目说明文档.md) | 目标、技术选型、架构、数据模型、接口、鉴权、部署 |
| 3 | [docs/前端设计文档.md](docs/前端设计文档.md) | 配色、字体、间距、组件、页面模板、响应式、无障碍 |
| 4 | [docs/开发规范.md](docs/开发规范.md) | 目录结构、Go/SQL/模板/CSS 约定、安全、发布流程、测试、踩坑速查 |
| 5 | [docs/学习笔记.md](docs/学习笔记.md) | 你的学习记录，每里程碑一节 |

新会话接手：读第 0 篇 → 扫一遍第 1 篇 → **问一句「还有哪块没说清楚」** → 再动手。
能省掉一整轮试错，也能省掉写完又推翻。

## 四条铁律

1. 公开页面对所有人统一，管理能力按身份分层。不做“按会员改首页内容”那套，差异只体现在多出来的管理入口和私密内容上，访客永远看到一个干净、完整、能读的站点。
2. 权限判断只在一个地方。`perm.Can(user, action, resource)` 一个纯函数，模板里禁止出现任何权限判断。数据查询必须带可见性过滤，只靠 UI 隐藏不算权限。
3. 改完即时生效。没有构建环节，后台改完文章，刷新前台就是新的。
4. 这项目是拿来学的。每个里程碑结束写一节学习笔记：学到什么、踩了什么坑、怎么修的。

## 这个包里有什么

只有文档。

```
mirage/
├── README.md
└── docs/
    ├── 交接文档.md
    ├── 项目说明文档.md
    ├── 前端设计文档.md
    ├── 开发规范.md
    ├── 决策记录.md
    └── 学习笔记.md
```

2026-10-28 起这个项目是从零重来。上一版留下的文章、插画、旧站代码都是那轮 AI 会话编出来的东西，已经全部作废，不作为迁移源。保留下来的只有两样：设计思路和整体技术选型。

也就是说，文档里写的每个表、每个接口，都是要你亲手写出来的，不是从旧代码搬过来的。

## 这个项目要练到什么

| 阶段 | 会学到什么 |
|---|---|
| HTTP 路由 | ServeMux 模式匹配、方法路由、路径参数、404/405 |
| 模板 | html/template 的自动转义，逻辑全在 Go 侧的边界怎么划 |
| 数据层 | database/sql、连接池、WAL、事务边界、SQLite 单写者 |
| 认证 | bcrypt、会话表、Cookie 签名、CSRF、踢人下线 |
| 权限 | RBAC 落地、可见性过滤、越权防护、为什么不能把判断写进模板 |
| 文件 | multipart 上传、流式落盘、路径穿越防护、下载鉴权 |
| 性能 | 首屏体积、gzip、字体子集化、缓存头、查询计划 |
| 运维 | systemd、Nginx 反代与缓存、部署回滚、日志 |

详见《项目说明文档》第 9 节的 10 个里程碑。M8 这期只落表结构和接口，不写上传代码，所以“文件 / multipart”那一行的实际动手时间要往后挪。

## 快速上手

```bash
# 0. 读文档
cat docs/决策记录.md
cat docs/项目说明文档.md

# 1. 初始化
go mod init mirage
go get modernc.org/sqlite github.com/yuin/goldmark \
       github.com/microcosm-cc/bluemonday golang.org/x/crypto

# 2. 起服务（开发模式，静态资源直接从 static/src 发，不用 Nginx）
go run ./cmd/mirage-server -dev
curl http://127.0.0.1:8091/healthz

# 3. 建表 + 迁移（仓库根的 migrations/001_init.sql，启动时自动应用）

# 4. 灌初始数据（板块、站点设置、owner 账号）
MIRAGE_OWNER_PASSWORD='你的初始密码' \
    go run ./cmd/seed -db data.db

# 5. 裁字体（发布时做一次，不是每次改文章都做）
python3 tools/subset-font.py \
    assets/fonts/LXGWWenKai-Regular.ttf \
    static/public/assets/fonts/lxgw-wenkai-subset.woff2 \
    templates/ content/ static/src/

# 6. 构建发布
make build && ./scripts/deploy.sh
```

配置是三级优先：命令行 flag > 环境变量 > 默认值。常用的几项：

```bash
mirage-server -addr 127.0.0.1:8091 -db /var/lib/mirage/data.db -dev
```

本地开发把 `.env.example` 复制成 `.env` 再 `source` 一下就行，密钥 `MIRAGE_SECRET` 用 `openssl rand -hex 32` 生成。

别跳阶段。M6（权限）是核心，“我看到的和别人看到的不一样”全靠它。每阶段结束先跑验收清单，再在 docs/学习笔记.md 写一节。

依赖只有四个：modernc.org/sqlite（纯 Go，不需要 gcc）、goldmark（Markdown）、bluemonday（渲染结果消毒）、golang.org/x/crypto（bcrypt）。其余全部标准库。gopkg.in/yaml.v3 只在 cmd/seed 里用，不进主二进制。

字体用霞鹜文楷，从 [lxgw/LxgwWenKai](https://github.com/lxgw/LxgwWenKai) 下载（OFL 协议）放到 assets/fonts/。

## 几个必看

### 踩坑速查

《开发规范》第 12 节有完整表格，这里挑最容易犯的：

- Nginx 哈希资源要用 `location ^~`，普通 `location` 会被正则抢走，immutable 静默失效
- `add_header` 在子层级会覆盖父层级全部 `add_header`，多个安全头要抽 snippet 每处 include
- 80 端口不能 301 跳 HTTPS，CDN 回源跟跳会成环
- SQLite 的 PRAGMA 必须写进 DSN，写成一次 Exec 只对当时那条连接生效
- 备份用 VACUUM INTO，裸拷 .db 会丢 WAL 里的事务
- 字体是发布期产物，新字不在子集里会退回系统字体，不是方框，下次发布重裁就统一了
- 裁字体要扫 templates/，Go 模板是 .html，不扫就一定缺字

### 服务器上不能动的东西

| 端口 | 服务 |
|---|---|
| 80 | Nginx，按 Host 分流（博客 / tiktok 站） |
| 443 | Nginx Proxy Manager，证书是废的，不要动 |
| 81/82 | Nginx Proxy Manager |
| 8080 | Trilium |
| 9000 | nginx-ui |
| 21 | vsftpd |
| 8090 | 占位页，旧站已删，保留作应急入口 |

- /usr/bin/node 是 v18.19.1，正被两个跑了 186 天的进程使用，绝不能覆盖
- Node 24 已随旧站删除，服务器上不再有 node24
- 内存 1.9G（2G swap 保留），新服务常驻内存目标 < 40 MB
- 已有 mavis 用户（uid 1003，sudo 组，仅密钥登录）

### 访问服务器

```bash
ssh -i ~/.ssh/id_ed25519 -o IdentitiesOnly=yes \
    -o StrictHostKeyChecking=accept-new \
    mavis@119.91.202.143
```

站点当前地址 http://www.miragewzy.online/，必须带 http://，原因见下。目前是占位页，旧站已下线，新站还没开工。

Nginx 配置已就位：/etc/nginx/sites-available/mirage（占位版）加 sites-enabled/mirage 软链。新站上线时改这个文件即可：location / 换成 proxy_pass http://127.0.0.1:8091;，静态资源段加 location ^~ /assets/ 加 immutable。只加这一个前缀，字体 CSS JS 全在里面。通用安全头 snippet 已备好：/etc/nginx/snippets/site-security.conf，5 个头，旧站的 astro-security.conf 改名而来。

## HTTPS 还悬着

443 被 NPM 占着、证书是废的，域名走 HTTPS 直接握手失败。

**现在的决定是暂不申请**，站点先用 http://。代价是分享链接的访问密码在传输里是明文——只适合给熟人看临时内容。

以前 certbot 签证书失败是因为挑战请求被 CDN 拦了，但**域名现在是直连服务器的，没接 CDN**，那条路值得再试一次（前提是先把 443 从 NPM 手里拿回来，它上面只剩一个死 Proxy Host）。详见《决策记录》D7。

## 备份：不做，这是有意接受的风险

只有一样东西丢不起：**数据库**。文章、板块、设置、账号、分享链接全在里面。

**但 git 里没有数据库**——`.db` 是二进制、每次都变，被 gitignore 了。服务器磁盘坏了或者误删了库，写过的文章就没了，git 的历史救不回来。唯一的例外是文章在本地写成 Markdown 文件再导入的，那 git 里天然有副本。

想补的时候用 `VACUUM INTO`（不能裸拷 `.db`，WAL 模式下会拷到缺数据的库），命令在《项目说明文档》第 8.4 节。

## 现状快照（2026-11-02）

| | 状态 |
|---|---|
| 旧站（Astro） | 已全部删除：源码、构建产物、nginx 配置、Node 24、部署脚本 |
| 上一版 AI 产出的资料 | 全部作废：文章、插画、旧参考代码，只留下设计思路和技术选型 |
| 域名 | miragewzy.online，直连服务器，**没接 CDN**；指向占位页 /var/www/placeholder/，不 502 |
| Nginx | 新配置 sites-available/mirage 已就位，等新站替换内容 |
| 新站（Go） | 尚未开工，本目录是完整的施工图 |
| 权限体系 | 待实现：perm.Can 两级判定 + 分享链接 |
| 评论 | 本期不做，表和接口都不留，见 D6 |
| 附件管理 | 本期不实现，只留表结构和接口，见 D11 |
| 备份 | 不做定时备份，风险已知，见 D21 |
| 代码托管 | GitHub 私有库 `mirage-xy/mirage-blog`，SSH over 443；发布是本地构建 + scp，服务器不拉代码 |
| 板块 | 先做 3 个：手记、关于、留言。以后要加养鸟、拍照这类板块，后台插一行记录 |
| 主题 | 方向定了（清新），具体样式待选，参考 Astro 主题库 |
| HTTPS | 暂不申请，先用 http://，见 D7 |
