// Copyright (c) 2024 mStar
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"github.com/mstarongithub/way2gay/repl"
)

func main() {
	repl := repl.NewRepl(nil, nil)
	repl.Run(msgHandler)
}

func msgHandler(in string, r *repl.Repl) (string, error) {
	return in, nil
}
