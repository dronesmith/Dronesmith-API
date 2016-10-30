/**
 * Dronesmith API
 *
 * Authors
 *  Geoff Gardner <geoff@dronesmith.io>
 *
 * Copyright (C) 2016 Dronesmith Technologies Inc, all rights reserved.
 * Unauthorized copying of any source code or assets within this project, via
 * any medium is strictly prohibited.
 *
 * Proprietary and confidential.
 */
 
package x25

import (
	// "hash"
	"testing"
)

func TestParsePacket(t *testing.T) {
	x := New()
	x.Sum16()
	// if h, ok := x.(*hash.Hash); !ok {
	//     t.Errorf("X25 does not implement hash.Hash")
	// }
}
