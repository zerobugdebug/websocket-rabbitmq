package main

import (
	"net/http"
	"os"
	"time"

	"github.com/gorilla/websocket"
	"github.com/streadway/amqp"
	"github.com/withmandala/go-log"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Maximum message size allowed from peer.
	maxMessageSize = 2048

	// Time allowed to read the next pong message from the peer.
	pongWait = 10 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Time to wait before force close on connection.
	closeGracePeriod = 10 * time.Second
)

//var addr = flag.String("addr", "localhost:8080", "http service address")

var upgrader = websocket.Upgrader{} // use default options
var loggerInfo = log.New(os.Stdout).WithDebug()

//var connectionRabbitMQ *amqp.Connection
//var channelRabbitMQ *amqp.Channel
//var  chan amqp.Delivery

//var conn = *amqp.Connection

func failOnError(err error, msg string) {
	if err != nil {
		loggerInfo.Fatalf("%s: %s", msg, err)
	}
}

/*
func initRabbitMQ() {
	connectionRabbitMQ, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer connectionRabbitMQ.Close()

	channelRabbitMQ, err := connectionRabbitMQ.Channel()
	failOnError(err, "Failed to open a channel")
	defer channelRabbitMQ.Close()

	messagesRabbitMQ, err := channelRabbitMQ.Consume(
		"WebsocketWorker", // queue
		"",                // consumer
		true,              // auto-ack
		false,             // exclusive
		false,             // no-local
		false,             // no-wait
		nil,               // args
	)
	failOnError(err, "Failed to register a consumer")
	//return conn, ch, nil
}
*/

func ping(ws *websocket.Conn) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		<-ticker.C
		loggerInfo.Info("Sending ping")
		if err := ws.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(writeWait)); err != nil {
			loggerInfo.Error("ping:", err)
			break
		}
	}

}

func internalError(ws *websocket.Conn, msg string, err error) {
	loggerInfo.Error(msg, err)
	ws.WriteMessage(websocket.TextMessage, []byte("Internal server error."))
}

func writeWebsocket(ws *websocket.Conn, chanMessage chan []byte) {
	loggerInfo.Info("Writer started")
	for {
		message := <-chanMessage
		err := ws.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			loggerInfo.Info("write:", err)
		}
		loggerInfo.Infof("Sent to websocket: %s", message)
	}
}

func readWebsocket(ws *websocket.Conn, chanMessage chan []byte) {
	defer ws.Close()
	ws.SetReadLimit(maxMessageSize)
	ws.SetReadDeadline(time.Now().Add(pongWait))
	ws.SetPongHandler(func(string) error { ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			loggerInfo.Error("read: ", err)
			break
		}
		loggerInfo.Infof("Recv from websocket: %s", message)
		chanMessage <- message
	}
}

func publishRabbitMQ(chanRabbitMQ *amqp.Channel, chanMessage chan []byte) {
	loggerInfo.Info("Publisher started")
	message := <-chanMessage
	err := chanRabbitMQ.Publish(
		"main",              // exchange
		"StartBattle.Start", // routing key
		false,               // mandatory
		false,               // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(message),
		})
	loggerInfo.Infof("Published to RabbitMQ: %s", message)
	failOnError(err, "Failed to publish a message")
}

/*func monitoringRabbitMQ(chanRabbitMQ *amqp.Channel, chanMessage chan []byte) {

	go func() {
		fmt.Printf("closing: %s", <-c.conn.NotifyClose(make(chan *amqp.Error)))
	}()

	chanRabbitMQ.n
	loggerInfo.Info("Publisher started")
	message := <-chanMessage
	err := chanRabbitMQ.Publish(
		"main",              // exchange
		"StartBattle.Start", // routing key
		false,               // mandatory
		false,               // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(message),
		})
	loggerInfo.Infof("Published to RabbitMQ: %s", message)
	failOnError(err, "Failed to publish a message")
}
*/
func consumeRabbitMQ(chanRabbitMQ *amqp.Channel, chanMessage chan []byte) {
	loggerInfo.Info("Consumer started")

	messagesRabbitMQ, err := chanRabbitMQ.Consume(
		"WebsocketWorker", // queue
		"",                // consumer
		true,              // auto-ack
		false,             // exclusive
		false,             // no-local
		false,             // no-wait
		nil,               // args
	)
	failOnError(err, "Failed to register a consumer")

	for d := range messagesRabbitMQ {
		loggerInfo.Infof("Consumed from RabbitMQ: %s", d.Body)
		chanMessage <- d.Body

	}
	loggerInfo.Error("Consumer crashed")

}

/*
func monitoredRabbitMQChannel(url string) (*amqp.Channel, error) {
	loggerInfo.Info("Creating monitored RabbitMQ connection...")
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}
	loggerInfo.Info("RabbitMQ connection: ", conn)

	loggerInfo.Info("Creating monitored RabbitMQ channel...")
	ch, err := conn.Channel()

	if err != nil {
		return nil, err
	}

	loggerInfo.Info("RabbitMQ channel: ", ch)

	channel := ch
	go func() {
		for {
			loggerInfo.Info("RabbitMQ channel monitoring started")
			reason, ok := <-channel.NotifyClose(make(chan *amqp.Error))
			loggerInfo.Warnf("RabbitMQ channel closed. Reason: %v, ok: %v", reason, ok)
			for {
				loggerInfo.Info("Reconnecting RabbitMQ connection...")
				time.Sleep(3 * time.Second)
				conn, err := amqp.Dial(url)
				if err == nil {
					loggerInfo.Info("RabbitMQ connection reconnected")
					for {
						loggerInfo.Info("Reconnecting RabbitMQ channel...")
						time.Sleep(3 * time.Second)
						ch, err := conn.Channel()
						if err == nil {
							loggerInfo.Info("RabbitMQ channel reconnected")
							channel = ch
							break
						}
						loggerInfo.Warnf("Can't reconnect RabbitMQ channel. Error: %v", err)
					}
					break
				}
				loggerInfo.Warnf("Can't reconnect RabbitMQ connection. Error: %v", err)

			}
		}
	}()
	return channel, nil
}
*/

func monitoredRabbitMQChannel(conn *amqp.Connection, pipe chan *amqp.Connection, id string) (*amqp.Channel, error) {
	loggerInfo.Info(id, "- Creating monitored RabbitMQ channel...")
	ch, err := conn.Channel()

	if err != nil {
		return nil, err
	}

	//	loggerInfo.Info("RabbitMQ channel: ", ch)

	channel := ch
	go func() {
		for {
			loggerInfo.Info(id, "- RabbitMQ channel monitoring started")
			reason, ok := <-channel.NotifyClose(make(chan *amqp.Error))
			loggerInfo.Warnf("%v - RabbitMQ channel closed. Reason: %v, ok: %v", id, reason, ok)
			//loggerInfo.Error("RabbitMQ connection from pipe: ", conn)
			for {
				loggerInfo.Info(id, "- Reconnecting RabbitMQ channel...")
				time.Sleep(3 * time.Second)
				conn := <-pipe
				ch, err := conn.Channel()
				if err == nil {
					loggerInfo.Info(id, "- RabbitMQ channel reconnected")
					channel = ch

					//Non-blocking push new connection back to pipe to chain new connection to other RabbitMQ channels
					select {
					case pipe <- conn:
					default:
					}

					break
				}
				loggerInfo.Warnf(id, "- Can't reconnect RabbitMQ channel. Error: %v", err)
				//loggerInfo.Info("RabbitMQ connection: ", conn)
			}
		}
	}()
	return channel, nil
}

func monitoredRabbitMQConnection(url string, pipe chan *amqp.Connection, id string) (*amqp.Connection, error) {
	loggerInfo.Info(id, "- Creating monitored RabbitMQ connection...")
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}
	//loggerInfo.Info("RabbitMQ connection: ", conn)

	connection := conn
	go func() {
		for {
			loggerInfo.Info(id, "- RabbitMQ connection monitoring started")
			reason, ok := <-connection.NotifyClose(make(chan *amqp.Error))
			loggerInfo.Warnf("%v - RabbitMQ connection closed. Reason: %v, ok: %v", id, reason, ok)
			for {
				loggerInfo.Info(id, "- Reconnecting RabbitMQ connection...")
				time.Sleep(3 * time.Second)
				conn, err := amqp.Dial(url)
				if err == nil {
					loggerInfo.Info(id, "- RabbitMQ connection reconnected")
					connection = conn
					//loggerInfo.Info("Reconnected RabbitMQ connection: ", conn)
					pipe <- conn //Send new connection info to the pipe
					break
				}
				loggerInfo.Warnf(id, "- Can't reconnect RabbitMQ connection. Error: %v", err)
			}
		}
	}()
	return connection, nil
}

func serveWs(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		loggerInfo.Error("upgrade:", err)
		return
	}
	defer ws.Close()

	//wsNetConnection := ws.UnderlyingConn()
	//localAddress := wsNetConnection.LocalAddr()
	//remoteAddress := wsNetConnection.RemoteAddr()
	pipeConnectionRabbitMQ := make(chan *amqp.Connection)

	connectionRabbitMQ, err := monitoredRabbitMQConnection("amqp://guest:guest@localhost:5672/", pipeConnectionRabbitMQ, "[Main]")
	if err != nil {
		loggerInfo.Error("monitoredRabbitMQConnection:", err)
		return
	} //failOnError(err, "Failed to connect to RabbitMQ")
	defer connectionRabbitMQ.Close()

	//channelRabbitMQ, err := connectionRabbitMQ.Channel()
	//failOnError(err, "Failed to open a channel")
	consumeChannelRabbitMQ, err := monitoredRabbitMQChannel(connectionRabbitMQ, pipeConnectionRabbitMQ, "[Consumer]")
	if err != nil {
		loggerInfo.Error("consumeChannelRabbitMQ error:", err)
		return
	}
	defer consumeChannelRabbitMQ.Close()

	publishChannelRabbitMQ, err := monitoredRabbitMQChannel(connectionRabbitMQ, pipeConnectionRabbitMQ, "[Publisher]")
	if err != nil {
		loggerInfo.Error("publishChannelRabbitMQ error:", err)
		return
	}
	defer publishChannelRabbitMQ.Close()

	//loggerInfo.Info("channelRabbitMQ: ", channelRabbitMQ)
	//loggerInfo.Info("*channelRabbitMQ: ", *channelRabbitMQ)
	//loggerInfo.Info("&channelRabbitMQ: ", &channelRabbitMQ)

	//forever := make(chan bool)

	go ping(ws)

	chanMesageFromRabbitMQ := make(chan []byte)
	go consumeRabbitMQ(consumeChannelRabbitMQ, chanMesageFromRabbitMQ)
	go writeWebsocket(ws, chanMesageFromRabbitMQ)

	//chanMesageFromRabbitMQ <- []byte(localAddress.Network())
	//chanMesageFromRabbitMQ <- []byte(localAddress.String())

	//chanMesageFromRabbitMQ <- []byte(remoteAddress.Network())
	//chanMesageFromRabbitMQ <- []byte(remoteAddress.String())

	chanMesageFromWebsocket := make(chan []byte)
	go publishRabbitMQ(publishChannelRabbitMQ, chanMesageFromWebsocket)
	//go pumpStdout(ws, outr, stdoutDone)
	readWebsocket(ws, chanMesageFromWebsocket)

	//<-forever
	/*
		for {
			_, message, err := ws.ReadMessage()
			if err != nil {
				loggerInfo.Info("read:", err)
				//break
			}
			loggerInfo.Infof("recv: %s", message)
	*/ //loggerInfo.Infof("mt: %s", mt)
	/*
		err = channelRabbitMQ.Publish(
			"main",              // exchange
			"StartBattle.Start", // routing key
			false,               // mandatory
			false,               // immediate
			amqp.Publishing{
				ContentType: "text/plain",
				Body:        []byte(message),
			})
		loggerInfo.Infof(" [x] Sent %s", message)
		failOnError(err, "Failed to publish a message")
	*/
	//		err = c.WriteMessage(mt, d.Body)
	//		if err != nil {
	//			loggerInfo.Info("write:", err)
	//		}
	//		loggerInfo.Infof("sent: %s", d.Body)

	//		failOnError(err, "Failed to register a consumer")

	//}
}

func serveHome(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "index.html")
}

func main() {
	//flag.Parse()
	//log.SetFlags(0)
	//loggerInfo := log.New(os.Stdout).WithDebug()

	http.HandleFunc("/ws", serveWs)
	http.HandleFunc("/", serveHome)
	loggerInfo.Fatal(http.ListenAndServe(":8080", nil))
}
