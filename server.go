package pubsub
// reference https://blog.csdn.net/xcl168/article/details/44355611
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