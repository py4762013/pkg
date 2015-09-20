// Copyright 2015, Cyrill @ Schumacher.fm and the CoreStore contributors
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

package email

import (
	"github.com/corestoreio/csfw/config"
	"github.com/go-gomail/gomail"
)

// newPlainDialer stubbed out for tests
var newPlainDialer func(host string, port int, username, password string) *gomail.Dialer = gomail.NewPlainDialer

var _ Dialer = (*gomailPlainDialer)(nil)

// gomailPlainDialer is a wrapper for the interface Dialer.
type gomailPlainDialer struct {
	*gomail.Dialer
}

// SetConfigReader noop method to comply with the interface Dialer.
func (gomailPlainDialer) SetConfigReader(config.Reader) {
	// noop
}