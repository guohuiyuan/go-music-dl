# music-lib

`music-lib` 是一个用 Go 编写的音乐平台能力库。

它不提供 UI，主要用来统一不同平台的歌曲、歌单、专辑搜索与解析能力，适合拿来做下载器、播放器、聚合工具或你自己的上层服务。

## 主要功能

支持网易云、QQ、酷狗、酷我这些主流平台，也能搞定汽水音乐、5sing 这些。具体支持情况如下：

| 平台       | 包名         | 搜索 | 下载 | 歌词 | 歌曲解析 | 歌单搜索 | 歌单推荐 | 歌单歌曲 | 歌单链接解析 | 专辑搜索 | 专辑歌曲 | 专辑链接解析 | 备注                                     |
| ---------- | ------------ | ---- | ---- | ---- | -------- | -------- | -------- | -------- | ------------ | -------- | -------- | ------------ | ---------------------------------------- |
| 网易云音乐 | `netease`    | ✅   | ✅   | ✅   | ✅       | ✅       | ✅       | ✅       | ✅           | ✅       | ✅       | ✅           | 支持 FLAC 无损                           |
| QQ 音乐    | `qq`         | ✅   | ✅   | ✅   | ✅       | ✅       | ✅       | ✅       | ✅           | ✅       | ✅       | ✅           | 支持 FLAC 无损                           |
| 酷狗音乐   | `kugou`      | ✅   | ✅   | ✅   | ✅       | ✅       | ✅       | ✅       | ✅           | ✅       | ✅       | ✅           | 支持普通歌曲 FLAC 无损                   |
| 酷我音乐   | `kuwo`       | ✅   | ✅   | ✅   | ✅       | ✅       | ✅       | ✅       | ✅           | ✅       | ✅       | ✅           |                                          |
| 咪咕音乐   | `migu`       | ✅   | ✅   | ✅   | ✅       | ✅       | ❌       | ✅       | ✅           | ✅       | ✅       | ✅           | 歌单歌曲使用 MIGUM3 接口                 |
| 千千音乐   | `qianqian`   | ✅   | ✅   | ✅   | ✅       | ⚠️       | ❌       | ✅       | ✅           | ✅       | ✅       | ✅           | 歌单搜索可能返回空，已知 ID/链接可解析   |
| 汽水音乐   | `soda`       | ✅   | ✅   | ✅   | ✅       | ✅       | ❌       | ✅       | ✅           | ✅       | ✅       | ✅           | 音频解密，支持短链和个人歌单             |
| 5sing      | `fivesing`   | ✅   | ✅   | ✅   | ✅       | ✅       | ❌       | ✅       | ✅           | ❌       | ❌       | ❌           |                                          |
| Jamendo    | `jamendo`    | ✅   | ✅   | ✅   | ✅       | ⚠️       | ❌       | ✅       | ✅           | ✅       | ✅       | ✅           | 歌单搜索可能返回空，公开歌单链接可解析   |
| JOOX       | `joox`       | ✅   | ✅   | ✅   | ✅       | ✅       | ❌       | ✅       | ✅           | ✅       | ✅       | ✅           | 歌单支持 OpenJOOX 接口和网页数据兜底     |
| Bilibili   | `bilibili`   | ✅   | ✅   | ✅   | ✅       | ✅       | ❌       | ✅       | ✅           | ❌       | ❌       | ❌           | 支持 FLAC 无损                           |
| Apple Music | `apple`     | ✅   | ⚠️  | ✅   | ✅       | ✅       | ❌       | ✅       | ✅           | ✅       | ✅       | ✅           | 下载仅 preview，完整需 gamdl 解密        |

> `⚠️` 表示方法已接入，但平台搜索接口结果不稳定；优先使用已知 ID 或链接解析。
> 本次从 `❌` 升级的能力已在 `go-music-dl` 侧通过集成测试验证；千千音乐和 Jamendo 的歌单搜索仍按不稳定能力标记。

个人化、登录和歌单分类能力支持情况：

| 平台       | 个人歌单 | 扫码登录 | 歌单分类 |
| ---------- | -------- | -------- | -------- |
| 网易云音乐 | ✅       | ✅       | ✅       |
| QQ 音乐    | ✅       | ✅ QQ / 微信 | ✅       |
| 酷狗音乐   | ✅       | ✅       | ✅       |
| 酷我音乐   | ❌       | ❌       | ✅       |
| 咪咕音乐   | ❌       | ❌       | ✅       |
| 千千音乐   | ❌       | ❌       | ✅       |
| 汽水音乐   | ✅       | ⚠️ 未调通 | ❌       |
| 5sing      | ❌       | ❌       | ❌       |
| Jamendo    | ❌       | ❌       | ❌       |
| JOOX       | ❌       | ❌       | ✅       |
| Bilibili   | ❌       | ✅       | ❌       |
| Apple Music | ❌      | ❌       | ✅       |

## Soda 扫码登录

`soda.CreateQRLogin` / `soda.CheckQRLogin` 目前仍处于调试状态。汽水音乐新版 PC 官方 passport 扫码流程依赖动态 `a_bogus` / `msToken` 风控签名，当前实现尚未调通完整登录链路，调用方不要把它作为稳定扫码登录能力暴露给用户。

已尝试按抓包还原 `check_qrconnect`、`send_code`、`validate_code`、MFA 后二次 `check_qrconnect` 的请求体和字段顺序，但真实轮询仍可能在 `new` / `scanned` 阶段触发 `error_code=7`。如果后续能实时生成官方签名，再恢复稳定支持。

## 安装

```bash
go get github.com/guohuiyuan/music-lib
```

## 使用示例

### 1. 搜歌并获取下载地址

```go
package main

import (
	"fmt"
	"log"

	"github.com/guohuiyuan/music-lib/kugou"
)

func main() {
	songs, err := kugou.Search("周杰伦")
	if err != nil {
		log.Fatal(err)
	}
	if len(songs) == 0 {
		fmt.Println("没有找到相关歌曲")
		return
	}

	url, err := kugou.GetDownloadURL(&songs[0])
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("下载地址:", url)
}
```

### 2. 搜索专辑并获取专辑歌曲

```go
package main

import (
	"fmt"
	"log"

	"github.com/guohuiyuan/music-lib/qq"
)

func main() {
	albums, err := qq.SearchAlbum("Taylor")
	if err != nil {
		log.Fatal(err)
	}
	if len(albums) == 0 {
		fmt.Println("没有找到相关专辑")
		return
	}

	album := albums[0]
	songs, err := qq.GetAlbumSongs(album.ID)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("专辑: %s, 共 %d 首歌\n", album.Name, len(songs))
}
```

### 3. 解析歌单链接

```go
package main

import (
	"fmt"
	"log"

	"github.com/guohuiyuan/music-lib/netease"
)

func main() {
	link := "https://music.163.com/#/playlist?id=123456"
	playlist, songs, err := netease.ParsePlaylist(link)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s 共有 %d 首歌\n", playlist.Name, len(songs))
}
```

### 4. 获取个人歌单

个人歌单需要登录 Cookie。支持的平台可以使用 `New(cookie)` 创建实例，再调用 `GetUserPlaylists`。目前支持网易云音乐、QQ 音乐、酷狗音乐和汽水音乐。

```go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/guohuiyuan/music-lib/qq"
)

func main() {
	q := qq.New(os.Getenv("QQ_COOKIE"))

	playlists, err := q.GetUserPlaylists(1, 20)
	if err != nil {
		log.Fatal(err)
	}
	if len(playlists) == 0 {
		fmt.Println("没有个人歌单")
		return
	}

	playlist := playlists[0]
	songs, err := q.GetPlaylistSongs(playlist.ID)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("歌单: %s, 共 %d 首歌\n", playlist.Name, len(songs))
}
```

QQ 音乐个人歌单里可能出现一些内部 ID：

- `profile:favorites`：我喜欢的歌曲。
- `profile:dir:<dirid>`：微信登录用户的个人目录歌单。

这些 ID 可以直接传给 `GetPlaylistSongs`，不需要自己转换成公开歌单 ID。

### 5. 扫码登录

已支持网易云、QQ、QQ 微信、酷狗、Bilibili 的扫码登录。登录成功后会返回 Cookie，建议由上层应用保存到配置或环境变量，不要硬编码在代码里。汽水音乐扫码登录暂未调通，请先手动获取 Cookie。

下面是 QQ 音乐微信扫码登录示例：

```go
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/qq"
)

func main() {
	session, err := qq.CreateWXQRLogin()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("请用微信扫描二维码:", session.ImageURL)

	for {
		time.Sleep(2 * time.Second)

		result, err := qq.CheckWXQRLogin(session.Key)
		if err != nil {
			log.Fatal(err)
		}

		switch result.Status {
		case model.QRLoginStatusWaiting, model.QRLoginStatusScanned:
			fmt.Println("登录状态:", result.Status)
		case model.QRLoginStatusSuccess:
			fmt.Println("登录成功，Cookie 长度:", len(result.Cookie))
			return
		case model.QRLoginStatusExpired, model.QRLoginStatusFailed:
			log.Fatalf("登录失败: %s", result.Message)
		}
	}
}
```

QQ 音乐默认扫码登录使用 `qq.CreateQRLogin` / `qq.CheckQRLogin`，微信扫码使用 `qq.CreateWXQRLogin` / `qq.CheckWXQRLogin`。如果想按类型切换，也可以使用 `qq.CreateQRLoginByType("qq")` / `qq.CheckQRLoginByType("qq", key)`，或 `qq.CreateQRLoginByType("wx")` / `qq.CheckQRLoginByType("wx", key)`。

### 6. 获取歌单分类

支持歌单分类的平台可以先获取分类，再按分类分页拉取歌单。

```go
package main

import (
	"fmt"
	"log"

	"github.com/guohuiyuan/music-lib/qq"
)

func main() {
	categories, err := qq.GetPlaylistCategories()
	if err != nil {
		log.Fatal(err)
	}
	if len(categories) == 0 {
		fmt.Println("没有分类")
		return
	}

	category := categories[0]
	playlists, err := qq.GetCategoryPlaylists(category.ID, 1, 20)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("分类: %s, 歌单数: %d\n", category.Name, len(playlists))
}
```

## 设计说明

- 独立性：每个平台包彼此独立，可以按需引入。
- 统一性：大部分平台统一返回 `Song` 和 `Playlist` 结构，便于上层复用。
- 可扩展性：新增平台时，按现有包结构补实现即可。

## 目录结构

```text
music-lib/
├── model/      # 通用数据结构
├── provider/   # 接口定义
├── netease/    # 各平台实现
├── qq/
├── kugou/
├── kuwo/
├── migu/
├── qianqian/
├── soda/
├── ...
└── README.md
```

## 许可证

本项目使用 `GNU Affero General Public License v3.0 (AGPL-3.0)`，详见 [LICENSE](LICENSE)。

## 免责声明

这个库就是写着玩、学技术的。大家用的时候遵守一下法律法规，不要拿去商用。下载的资源 24 小时内删掉。
