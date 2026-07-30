//	Copyright 2025 TuneIn, Inc. All rights reserved.
//
// Use of this source code is governed by Apache License 2.0
// license that can be found in the LICENSE file.
package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type CacheItemSuite struct {
	suite.Suite
}

func TestCacheItemSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, &CacheItemSuite{})
}

// TestExpired ensures correct expired value returning
func (s *CacheItemSuite) TestExpired() {
	testCases := []struct {
		title   string
		expired bool
		added   time.Time
		ttl     time.Duration
	}{
		{
			title:   "True",
			added:   time.Now().Add(-10 * time.Second),
			expired: true,
			ttl:     5 * time.Second,
		},
		{
			title:   "False",
			added:   time.Now(),
			expired: false,
			ttl:     5 * time.Second,
		},
		{
			title:   "No TTL",
			added:   time.Now(),
			expired: false,
			ttl:     0,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			var (
				validate = s.Assert()
				ci       = &cacheItem[string]{}
			)
			ci.added = tc.added
			ci.ttl = tc.ttl
			ci.val = "hello"
			validate.Equal(ci.expired(), tc.expired)
		})
	}
}
