package main

import (
	"log"
	"net/rpc"

	"github.com/nickvanw/ircx"
	"github.com/paked/configure"
	"github.com/sorcix/irc"
)

var (
	conf = configure.New()

	name     = conf.String("name", "roomer", "The nick of your bot")
	server   = conf.String("server", "chat.freenode.net:6667", "Host:Port to connect to")
	channels = conf.String("chan", "#roomtest", "Host:Port to connect to")
)

func main() {
	conf.Use(configure.NewFlag())
	conf.Use(configure.NewEnvironment())

	conf.Parse()

	bot := ircx.Classic(*server, *name)
	if err := bot.Connect(); err != nil {
		log.Panicln("Unable to connect to that IRC server", err)
	}

	bot.HandleFunc(irc.RPL_WELCOME, registerHandler)
	bot.HandleFunc(irc.PING, pingHandler)
	bot.HandleFunc(irc.PRIVMSG, msgHandler)

	bot.HandleLoop()
}

func msgHandler(s ircx.Sender, m *irc.Message) {
	log.Println(m.Params, m.Trailing)
}

func registerHandler(s ircx.Sender, m *irc.Message) {
	s.Send(&irc.Message{
		Command: irc.JOIN,
		Params:  []string{*channels},
	})
}

func pingHandler(s ircx.Sender, m *irc.Message) {
	s.Send(&irc.Message{
		Command:  irc.PONG,
		Params:   m.Params,
		Trailing: m.Trailing,
	})
}
