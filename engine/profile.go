package engine

import (
	"github.com/Ivlyth/process-bandwidth/pkg/profile"
)

var SnapShotProfileCounter = &profile.Counter{}
var RefreshNetConnectionsProfileCounter = &profile.Counter{}
var EventProcessProfileCountersMap map[string]*profile.Counter
var PerfReaderReadProfileCounter = &profile.Counter{}
var PerfReaderLostProfileCounter = &profile.Counter{}
var RingPoolProfileCounter = &profile.Counter{}
