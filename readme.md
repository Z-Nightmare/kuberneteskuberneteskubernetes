    千里烽烟连夜急 一骑星光踏月还
    
<img src="/docs/Hermes.webp" alt="image" style="width:50%; height:50%;" />

## 流程设计

### MVP

1. ✓前端获取所在经纬度，附带原始待翻译信息
1. ✓根据经纬度使用 Haversine 公式计算距离配置中最接近的城市
1. ✓AI翻译
4. ✓配置中加一个特性开关，开启的时候，多国语言会写入到output目录里面

## model

```go
type Hermes interface {
	Translate(source Language, location Location) (target []Language, err error)
}
```

## api

http://localhost:8080/translate

- request

```bash
curl 'https://your-api-endpoint.com/translate' \
  -H 'sec-ch-ua-platform: "macOS"' \
  -H 'Referer;' \
  -H 'User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36' \
  -H 'sec-ch-ua: "Chromium";v="136", "Google Chrome";v="136", "Not.A/Brand";v="99"' \
  -H 'DNT: 1' \
  -H 'Content-Type: application/json' \
  -H 'sec-ch-ua-mobile: ?0' \
  --data-raw '{"text":"我要学习所有语言","location":{"latitude":63.200906186376439,"longitude":112.20250603795485}}'
```

- response

```json
{
    "code": 200,
    "message": "- 中文: 我要学习所有语言  \n- 英语: I want to learn all languages  \n- 日语: 私はすべての言語を学びたい",
    "cost": 5016763792
}
```

## web

<img src="/docs/example.png" alt="image" style="width:50%; height:50%;" />

## usage

```BASH
  echo 'DEEPSEEK_API_KEY=sk-xxx' >> .env
  #//optional:
  code cmd/web/.config.yaml
  cd cmd/web;make run

  open http://localhost:8080/translate
```

## todo

1. 缓存用户的地理位置
1. 跳过与当前语言一致的目标语言
2. 发布社交媒体
3. 改点bug
