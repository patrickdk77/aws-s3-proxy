package service

import "sync"

func resetAWS() {
	cfgOnce = sync.Once{}
	clients = sync.Map{}
}
