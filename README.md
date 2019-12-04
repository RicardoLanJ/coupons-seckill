# coupons-seckill
#### 数据库地址、端口、密码配置

    redis设置 "localhost:6379"
    mysql设置 "root:admin@tcp(127.0.0.1:3306)/msxt?charset=utf8"

#### 下载所需库 go get github.com/xxxx/xxxx

    github.com/gin-gonic/gin
    github.com/go-sql-driver/mysql
    github.com/sirupsen/logrus
    github.com/dgrijalva/jwt-go
    github.com/go-redis/redis

#### 运行
    > 启动mysql 和 redis
    > go build   //修改后重新编译
    > sproxy.exe  //现在是debug mod 

文件夹怎么放在go的$GOPATH下，可以搜一下

#### 日志
    logrus.log

#### 测试
用postman这个工具。写好的一些测试请求，在/postman下
auth头要先登录再去header那里复制，不懂可以查下jwt (JSON Web Token)

### 整体架构
go的gin框架处理请求，并发请求gin会开goroutine来处理，无需担。

其实没必要用mysql，一开始用了懒得改了

redis 用于三个功能 
+ key = 优惠卷名字 value=剩余
+ key = 优惠卷名字+'info' hash = {stock, description, ...}
+ key = 用户名（包括商家） set = 优惠卷名字集合


问了ta优惠卷名字唯一，即一种优惠卷只存在于一个商家

redis作消息队列，用gocelery开worker处理请求，服务器和数据库都在同一台主机，所以没必要。

redis在抢购时用乐观锁处理并发，optimisticLockSK函数不知道有没有写对。不知道怎么测并发。

后面log懒得写了，只log了fatal

总之看看源码就懂了



