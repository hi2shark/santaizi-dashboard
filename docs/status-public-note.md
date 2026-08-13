# 公开备注字段

公开站读取服务器 `public_note` JSON（HTTP 与 `/ws/v2/public/runtime` 同形）。Admin 在服务器编辑器中配置。

## billingDataMod

| 字段 | 说明 |
| ---- | ---- |
| amount | 价格；`0` 免费；`-1` 按量 |
| cycle | 月/年/季/半年等（也接受英文别名） |
| endDate | 到期；`0000-00-00` 表示长期 |
| autoRenewal | `"1"` 时按周期推算下一到期 |
| startDate | 可选 |

## planDataMod

| 字段 | 说明 |
| ---- | ---- |
| bandwidth | 带宽文案 |
| trafficVol / trafficType | 流量额度；`1` 单向出 / `3` 单向取最大 / 默认双向 |
| IPv4 / IPv6 | `"1"` 启用 |
| networkRoute / extra | 标签（逗号分隔） |

## customData

| 字段 | 说明 |
| ---- | ---- |
| location | 点阵地图 / 地球仪位置码（如 `HKG`） |
| slogan | 标语 |
| orderLink | 购买链接（`&` 需 URL 编码） |
| flag | 旗帜代码 |
| buyBtnText / buyBtnIcon | 购买按钮文案与 Remix 图标类名 |
