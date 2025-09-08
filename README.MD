# 安洁露邮件/包裹溯源查询系统（技术测试）

该系统可用于追溯明信片、信件、包裹等发出时的状态  
可为货品创建一个唯一ID，自行打印后张贴于包裹上，可供收件方查询。  
用户收件后可通过扫描邮件二维码或手动输入ID进行查询。  
创建时支持设置查询密码及附图。

## 部署

开发环境
> docker compose up -d app

生产环境
> docker compose up -d app-prod

## 要求

1. ``Geolite2`` mmdb 数据库（放置到``/geo_data``）下
2. ``docker-compose``中挂载 ``/data`` 作为数据保存位置
3. 注册 ``Cloudflare Turnstile`` 验证码，填写``.env``中的环境变量

## Todo

1. 支持设置查询条件 Before/After 指定日期
2. 前后端分离

## 开源许可证
MIT LICENSE