想用golang写1个服务，这个服务主要用于不方便使用消息队列服务的业务方，推送消息并回调到某个http接口，实现异步操作和延时处理某个消息


架构可以参考 https://github.com/bxcodec/go-clean-arch

前端使用react+shadcn 查看later任务的调度信息

golang 使用gin web框架