package main

import (
	"net"

	"github.com/jxsl13/twapi/browser"
)

type byPlayerCountDescending []browser.ServerInfo

func (a byPlayerCountDescending) Len() int           { return len(a) }
func (a byPlayerCountDescending) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byPlayerCountDescending) Less(i, j int) bool { return len(a[i].Players) > len(a[j].Players) }

type byAddress []*net.UDPAddr

func (a byAddress) Len() int           { return len(a) }
func (a byAddress) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byAddress) Less(i, j int) bool { return a[i].String() < a[j].String() }
