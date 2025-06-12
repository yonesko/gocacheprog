package main

import "sync/atomic"

type (
	stat struct {
		getCmd int64
		putCmd int64
	}
)

func metric(s *stat, h handlerFunc) handlerFunc {
	return func(req Request) (Response, error) {
		switch req.Command {
		case CmdGet:
			atomic.AddInt64(&s.getCmd, 1)
		case CmdPut:
			atomic.AddInt64(&s.putCmd, 1)
		}
		return h(req)
	}
}
