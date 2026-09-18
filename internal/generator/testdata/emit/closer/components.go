// Copyright the original author or authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
