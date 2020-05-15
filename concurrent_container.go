package main

import (
	"errors"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// NewConcurrentServerList creates a new empty list with capacity empty slots
func NewConcurrentServerList(capacity int) *ConcurrentServerList {
	return &ConcurrentServerList{list: make([]*net.UDPAddr, 0, capacity)}
}

// ConcurrentServerList allows for concurrent access
type ConcurrentServerList struct {
	sync.Mutex
	list []*net.UDPAddr
}

// Len of the list
func (c *ConcurrentServerList) Len() int {
	c.Lock()
	defer c.Unlock()

	return len(c.list)
}

// Add adds only unique new servers to the list
func (c *ConcurrentServerList) Add(address string) error {
	matches := extractIPRegex.FindStringSubmatch(strings.TrimSpace(address))

	if len(matches) != 3 {
		return errors.New("invalid address format")
	}

	IP := net.ParseIP(matches[1])
	if IP == nil {
		return errors.New("invalid IP format")
	}

	port, err := strconv.Atoi(matches[2])
	if err != nil {
		return errors.New("invalid port format")
	}
	if port <= 1024 {
		return errors.New("port should be bigger than 1024")
	}

	c.Lock()
	defer c.Unlock()

	for _, s := range c.list {
		if s.IP.Equal(IP) && s.Port == port {
			return errors.New("server address already exists")
		}
	}

	c.list = append(c.list, &net.UDPAddr{IP: IP, Port: port})
	return nil
}

// List returns a copy of the list
func (c *ConcurrentServerList) List() (list []*net.UDPAddr) {

	c.Lock()
	list = make([]*net.UDPAddr, 0, len(c.list))

	for _, s := range c.list {
		s := s
		list = append(list, s)
	}
	c.Unlock()
	return
}

// SortedList returns a copy of the list
func (c *ConcurrentServerList) SortedList() (list []*net.UDPAddr) {

	list = c.List()
	sort.Sort(byAddress(list))

	return
}

// Detete an entry from the list
func (c *ConcurrentServerList) Delete(address string) error {
	matches := extractIPRegex.FindStringSubmatch(strings.TrimSpace(address))

	if len(matches) != 3 {
		return errors.New("invalid address format")
	}

	IP := net.ParseIP(matches[1])
	if IP == nil {
		return errors.New("invalid IP format")
	}

	port, err := strconv.Atoi(matches[2])
	if err != nil {
		return errors.New("invalid port format")
	}
	if port <= 1024 {
		return errors.New("port should be bigger than 1024")
	}

	position := -1

	c.Lock()
	defer c.Unlock()
	for idx, s := range c.list {
		if s.IP.Equal(IP) && s.Port == port {
			position = idx
		}
	}

	if position < 0 {
		return errors.New("IP not found")
	}

	c.list = append(c.list[:position], c.list[position+1:]...)

	return nil
}
