package main

import (
	"io"
	"net/rpc"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/bigroom/vision/tunnel"
	"github.com/evalphobia/logrus_sentry"
	"github.com/nickvanw/ircx"
	"github.com/paked/configure"
	"github.com/sorcix/irc"
)

var (
	client *rpc.Client

	conf           = configure.New()
	reconnectDelay = time.Duration(2)

	name       = conf.String("name", "roomer", "The nick of your bot")
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

	_, err = logrus_sentry.NewSentryHook(*sentryDSN, []log.Level{
		log.PanicLevel,
		log.FatalLevel,
		log.ErrorLevel,
	})

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
		fields := log.Fields{
			"name":    "RPC",
			"host":    args.Host,
			"channel": args.Channel,
		}

		log.Debug("Trying to send message...")
		err := client.Call("Message.Dispatch", &args, &reply)
		if err == nil {
			return nil
		}

		log.WithFields(fields).Error(err)

		log.Error("Couldnt send message, trying reconnect")

		if err := connectRPC(); err != nil {
			log.WithFields(fields).Error(err)
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
		log.WithFields(log.Fields{
			"channel": *channels,
			"host":    *serverName,
		}).Error(err)
	}
}

func pingHandler(s ircx.Sender, m *irc.Message) {
	err := s.Send(&irc.Message{
		Command:  irc.PONG,
		Params:   m.Params,
		Trailing: m.Trailing,
	})

	if err != nil {
		log.Error(err)
	}
}

// reconnect will continuosly attempt to reconnect to the RPC server
func reconnect(f func() error, name string) {
	err := f()
	if err == nil {
		return
	}

	fields := log.Fields{
		"name": name,
	}

	log.WithFields(fields).Error("Was disconnected from server")

	delay := 2
	for {
		timer := time.NewTimer(time.Second * time.Duration(delay) / 2)
		<-timer.C

		delay *= delay

		err = f()
		if err == nil {
			log.WithFields(fields).Info("Reconnected to server")
			return
		}

		log.WithFields(fields).Error(err)

		if !isNetworkError(err) {
			log.WithFields(log.Fields{
				"name": name,
				"type": "non-network",
			}).Error(err)
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
