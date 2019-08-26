一直对Redis里面的pub/sub很好奇，于是用golang做下简单的实现,做个原型，效果如图：
![publish/subscribe golang模拟实现](https://img-blog.csdnimg.cn/20190826164620120.gif)

## 大体模型
对pub/sub 模型来说，会有这么两条通路：

- **subscribe** 客户端主动链接，并订阅对应topic，此时状态信息被服务器记录下来
- **publish**  服务器根据topic主动进行数据推送，推送过程中就用到了subscribe时记录到的客户端连接状态信息

## 实现

**channel.go**
```go
package pubsub

import (
	"sync"
	"fmt"
	"sync/atomic"
	"net"
	"bufio"
)

type Channel struct {
	Name string
	clients map[int]*Client
	sync.RWMutex
	waitGroup sync.WaitGroup
	messageCount uint64
	exitFlag int32
}

func NewChannel(channelName string) *Channel {
	return &Channel{
		Name:channelName,
		clients:make(map[int]*Client),
	}
}

func (this *Channel) AddClient(client *Client) bool {
	this.RLock()
	_, found := this.clients[client.Id]
	this.RUnlock()

	this.Lock()
	if !found {
		this.clients[client.Id] = client
	}
	this.Unlock()
	return found
}

func (this *Channel) DeleteClient(client *Client) int{
	var remain int
	// 整理输出信息
	this.Lock()
	delete(this.clients, client.Id)
	this.Unlock()

	this.RLock()
	remain = len(this.clients)
	this.RUnlock()

	return remain
}

func (this *Channel) Notify(message string) bool {
	this.RLock()
	defer this.RUnlock()

	for clientid, client := range this.clients {
		this.PutMessage(clientid, message)
		go handleResponse(client.conn, message)
	}
	this.waitGroup.Done()
	return true
}

func handleResponse(conn net.Conn, message string) {
	// todo alive check
	fmt.Println("handleResponse: ", message)
	writter := bufio.NewWriter(conn)
	writter.WriteString("[CONSUME MESSAGE] " + message)
	writter.Flush()

}

func (this *Channel) Wait() {
	//this.waitGroup.Wait()
}
func (this *Channel) Exit() {
	if !atomic.CompareAndSwapInt32(&this.exitFlag, 0, 1) {
		return
	}
	this.Wait()
}

func (this *Channel) Exiting() bool {
	return atomic.LoadInt32(&this.exitFlag) == 1
}

func (this *Channel) PutMessage(clientid int, message string) {
	this.RLock()
	defer this.RUnlock()

	if this.Exiting() {
		return
	}
	fmt.Println("{", this.Name, "}[", clientid, "] ", message)
	atomic.AddUint64(&this.messageCount, 1)
	return
}
```

** server.go**
```go
package pubsub

import (
	"sync"
		"net"
	"strconv"
	"bufio"
	"fmt"
	"strings"
	)


type Client struct {
	Id int
	Ip string
	conn net.Conn
}

type Server struct {
	// map[Channel.Name]*Channel 一个channel会被很多个client给subscribe了
	Bucket map[string]*Channel
	sync.RWMutex
}


func NewServer() *Server {
	return &Server{
		Bucket:make(map[string]*Channel),
	}
}

func (this *Server) Run(host string, port int) error {
	address := host + ":" + strconv.Itoa(port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		go this.HandleRequest(conn)
	}
}


func (this *Server) HandleRequest(conn net.Conn) {
	//defer conn.Close()
	for {
		bytes, _, _ := bufio.NewReader(conn).ReadLine()
		content := strings.Trim(strings.Trim(string(bytes), " "), "\n")
		fmt.Println(fmt.Sprintf("request string: [%s]", content))
		writter := bufio.NewWriter(conn)
		if content == "subscribe" {
			address := conn.RemoteAddr().String()
			splits := strings.Split(address, ":")
			clientid, _ := strconv.Atoi(splits[1])
			client := &Client{
				Id:   clientid,
				Ip:   splits[0],
				conn: conn,
			}
			topic := "hello"
			this.Subscribe(client, topic)
			writter.WriteString(address)
		} else if content == "publish" {
			message := "PUBLISH MESSAGE"
			topic := "hello"
			this.PublishMessahe(topic, message)
		}else if content == "quit" {
			content = "client quited"
			break
		}else {
			fmt.Println("common chat " + content)
			writter.WriteString(content + "\n")
			writter.Flush()
		}
		writter.WriteString(content + "\n")
	}
}


func (this *Server)Subscribe(client *Client, channelName string) {
	this.RLock()
	channel, found := this.Bucket[channelName]
	this.RUnlock()

	if found {
		channel.AddClient(client)
	}else{
		// create a new channel, add this client
		newchannel := NewChannel(channelName)
		newchannel.AddClient(client)
		this.Lock()
		this.Bucket[channelName] = newchannel
		this.Unlock()
	}
}

func (this *Server) Ubsubscribe(client *Client, channelName string) {
	this.RLock()
	channel, found := this.Bucket[channelName]
	this.RUnlock()

	if found {
		remain := channel.DeleteClient(client)
		if remain == 0 {
			channel.Exit()
			this.Lock()
			delete(this.Bucket, channelName)
			this.Unlock()
		}
	}
}

func (this *Server) PublishMessahe(channelName, message string) (bool, string) {
	this.RLock()
	channel, found := this.Bucket[channelName]
	defer this.RUnlock()
	if !found {
		return false, "channelName 不存在"
	}
	channel.waitGroup.Add(1)
	go channel.Notify(message)
	channel.waitGroup.Wait()
	return true, ""
}
```

**main.go**
```go
package main

import (
	"github.com/guoruibiao/pubsub"
	"log"
	"fmt"
)

func main() {
	server  := pubsub.NewServer()
	fmt.Println("server running...")
	err := server.Run("localhost", 8080)
	if err != nil {
		log.Fatal(err)
	}


}
```

## 测试步骤
首先肯定是先把服务跑起来啦，如下：

```
$ go run main.go
server running...
```

然后是客户端链接测试，因为底层实现是用的tcp链接，所以可以借助netcat，这样就不用单独再写golang的客户端连接了。

打开第一个终端, 输入subscribe
```
nc localhost 8080
subscribe
```
打开第二个终端，输入subscribe

```
nc localhost 8080
subscribe
```
打开第三个终端，输入publish

```
nc localhost 8080
publish
```

就可以看到，第一、第二个终端有来自服务器的publish数据推送了，具体可以查看上面的GIF图。


## TODO

- [ ] 添加flag库，以支持命令行任意数据的发送
- [ ] 检测客户端是否alive，以避免对链接关闭了的tcp链接触发“写”操作。
- [ ] 使用channel，减少对net.Conn对象的分散调用。 

## 参考链接

[简单的订阅发布机制实现(Golang)](https://blog.csdn.net/xcl168/article/details/44355611)
