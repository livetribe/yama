// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package closer

import (
	"context"

	"github.com/google/wire"
)

// Conn stands in for a third-party type. It declares Close and no capability.
type Conn struct{}

func (*Conn) Close() error { return nil }

// Pool is built by a wire.Struct entry. It declares Close on the pointer.
type Pool struct{ Conn *Conn }

func (*Pool) Close() error { return nil }

// Out declares Close, and its provider states that the lifecycle does not
// close it.
type Out struct{}

func (*Out) Close() error { return nil }

// Log declares Close, and its provider carries no directive. A run warns about
// it.
type Log struct{}

func (*Log) Close() error { return nil }

type Root struct {
	pool *Pool
	out  *Out
	log  *Log
}

// NewConn provides the connection.
//
//yama:closer
func NewConn() *Conn { return &Conn{} }

// NewOut provides the output.
//
//yama:noclose
func NewOut() *Out { return &Out{} }

func NewLog() *Log { return &Log{} }

func NewRoot(pool *Pool, out *Out, log *Log) *Root {
	return &Root{pool: pool, out: out, log: log}
}

func (*Root) Start(context.Context) error { return nil }

var Set = wire.NewSet(NewConn, NewOut, NewLog, NewRoot,
	//yama:closer
	wire.Struct(new(Pool), "*"))
