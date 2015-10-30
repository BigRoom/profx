package main

import (
	"io"
	"net/rpc"
	"time"

	"github.com/bigroom/vision/tunnel"
	"github.com/getsentry/raven-go"
	"github.com/nickvanw/ircx"
	"github.com/paked/configure"
	log "github.com/sirupsen/logrus"
	"github.com/sorcix/irc"
)

var (
	client *rpc.Client
	sentry *raven.Client

	conf           = configure.New()
	reconnectDelay = time.Duration(2)

	name       = conf.String("name", "roomer", "The nick of your bot")
	server     = conf.String("server", "chat.freenode.net:6667", "Host:Port for the bot to connect to")
	serverName = conf.String("server-name", "chat.freenode.net:6667", "Host:Port for others to connect to")
	channels   = conf.String("chan", "#roomtest", "Host:Port to connect to")
	dispatch   = conf.String("dispatch", "localhost:8080", "Where to dispatch things")
	sentryDSN  = conf.String("sentry-dsn", "", "The sentry DSN")
)

func main() {
	var err error

	conf.Use(configure.NewFlag())
	conf.Use(configure.NewEnvironment())

	conf.Parse()

	sentry, err = raven.NewClient(*sentryDSN, nil)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Fatal("Unable to get connect to sentry")

		return
	}

	bot := ircx.Classic(*serverName, *name)

	log.WithFields(log.Fields{
		"address": *serverName,
	}).Info("Connecting to the IRC server")

	reconnect(bot.Connect, "IRC")

	bot.HandleFunc(irc.RPL_WELCOME, registerHandler)
	bot.HandleFunc(irc.PING, pingHandler)
	bot.HandleFunc(irc.PRIVMSG, msgHandler)

	log.WithFields(log.Fields{
		"address": *dispatch,
	}).Info("Connecting to RPC server")

	reconnect(connectRPC, "RPC")

	bot.HandleLoop()
}

func msgHandler(s ircx.Sender, m *irc.Message) {
	log.WithFields(log.Fields{
		"params":   m.Params,
		"trailing": m.Trailing,
	}).Info("Sending messsage")

	var reply tunnel.MessageReply
	args := tunnel.MessageArgs{
		From:    m.Name,
		Content: m.Trailing,
		Time:    time.Now(),
		Host:    *serverName,
		Channel: m.Params[0],
	}

	reconnect(func() error {
		log.Debug("Trying to send message...")
		err := client.Call("Message.Dispatch", &args, &reply)
		if err == nil {
			return nil
		}

		sentry.CaptureErrorAndWait(err, nil)

		log.Error("Couldnt send message, trying reconnect")

		if err := connectRPC(); err != nil {
			sentry.CaptureErrorAndWait(err, nil)
			return err
		}

		return err
	}, "RPC")

	if !reply.OK {
		log.Error("Reply was not ok")
	}
}

func registerHandler(s ircx.Sender, m *irc.Message) {
	log.Debug("Registered")
	err := s.Send(&irc.Message{
		Command: irc.JOIN,
		Params:  []string{*channels},
	})

	if err != nil {
		sentry.CaptureErrorAndWait(err, nil)
	}
}

func pingHandler(s ircx.Sender, m *irc.Message) {
	err := s.Send(&irc.Message{
		Command:  irc.PONG,
		Params:   m.Params,
		Trailing: m.Trailing,
	})

	if err != nil {
		sentry.CaptureErrorAndWait(err, nil)
	}
}

// reconnect will continuosly attempt to reconnect to the RPC server
func reconnect(f func() error, name string) {
	err := f()
	if err == nil {
		return
	}

	log.WithFields(log.Fields{
		"name": name,
	}).Error("Was disconnected from server")

	delay := 2
	for {
		timer := time.NewTimer(time.Second * time.Duration(delay) / 2)
		<-timer.C

		delay *= delay

		err = f()
		if err == nil {
			log.WithFields(log.Fields{
				"name": name,
			}).Info("Reconnected to server")
			return
		}

		sentry.CaptureErrorAndWait(err, nil)

		if !isNetworkError(err) {
			log.WithFields(log.Fields{
				"name":  name,
				"error": err,
			}).Error("Got non-network error")
		}
	}
}

func isNetworkError(err error) bool {
	return err == rpc.ErrShutdown || err == io.EOF || err == io.ErrUnexpectedEOF
}

func connectRPC() error {
	var err error
	client, err = rpc.DialHTTP("tcp", *dispatch)

	return err
}
