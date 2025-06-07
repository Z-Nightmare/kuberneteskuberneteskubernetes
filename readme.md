    千里烽烟连夜急 一骑星光踏月还
    
![image](/docs/Hermes.webp)

## todo

1. 获取所在位置，根据所在位置自动建立对应的目标语言 k-v map
1. 调用翻译api完成目标语言翻译
1. 分治算法
1. 多线程调用翻译api
1. 整理目标语言
1. 得到结果

## model

```go
type Hermes interface {
	Translate(source Language, location Location) (target []Language, err error)
}
```

## api

```cmd
curl 'https://your-api-endpoint.com/translate' \
  -H 'sec-ch-ua-platform: "macOS"' \
  -H 'Referer;' \
  -H 'User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36' \
  -H 'sec-ch-ua: "Chromium";v="136", "Google Chrome";v="136", "Not.A/Brand";v="99"' \
  -H 'DNT: 1' \
  -H 'Content-Type: application/json' \
  -H 'sec-ch-ua-mobile: ?0' \
  --data-raw '{"text":"fff","location":{"latitude":63.200906186376439,"longitude":112.20250603795485}}'
```

```json
{
  "text": "Hello world",
  "location": {
    "latitude": 31.2304,
    "longitude": 121.4737
  }
}
```
