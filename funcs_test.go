//	Copyright 2025 TuneIn, Inc. All rights reserved.
//
// Use of this source code is governed by Apache License 2.0
// license that can be found in the LICENSE file.
package cache

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type CacheFuncsSuite struct {
	suite.Suite
}

func TestCacheFuncsSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, &CacheFuncsSuite{})
}

// TestLoaderExpireFunc ensures loader expiration callback called correctly
func (s *CacheFuncsSuite) TestLoaderExpireFunc() {
	errLoad := errors.New("some error")
	testCases := []struct {
		title   string
		seed    bool
		key     int
		want    bool
		wantErr error
	}{
		{title: "ReloadsExpiredItem", seed: true, key: 100, want: true},
		{title: "ReturnsLoaderError", seed: false, key: 1, want: false, wantErr: errLoad},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			validate := s.Assert()
			var calls atomic.Int32
			cc := New[int, bool](time.Nanosecond)
			if tc.seed {
				cc.Set(tc.key, true)
			}
			cc.LoaderExpireFunc(func(key int) (bool, *time.Duration, error) {
				calls.Add(1)
				if key == 100 {
					return true, nil, nil
				}
				return false, nil, errLoad
			})
			validate.Zero(calls.Load())

			res, err := cc.Get(tc.key)
			validate.ErrorIs(err, tc.wantErr)
			validate.Equal(tc.want, res)
			validate.Equal(int32(1), calls.Load())
		})
	}
}

func (s *CacheFuncsSuite) TestLoaderConcurrentMisses() {
	testCases := []struct {
		title     string
		callers   int
		loadDelay time.Duration
		key       int
	}{
		{title: "ConcurrentMissesShareOneLoad", callers: 8, loadDelay: 50 * time.Millisecond, key: 21},
	}

	for _, tc := range testCases {
		s.Run(tc.title, func() {
			validate := s.Assert()
			var calls atomic.Int32
			cc := New[int, int](time.Minute)
			cc.LoaderFunc(func(key int) (int, error) {
				calls.Add(1)
				time.Sleep(tc.loadDelay)
				return key * 2, nil
			})

			var wg sync.WaitGroup
			for range tc.callers {
				wg.Go(func() {
					v, err := cc.Get(tc.key)
					validate.NoError(err)
					validate.Equal(tc.key*2, v)
				})
			}
			wg.Wait()

			validate.Equal(int32(1), calls.Load())
		})
	}
}
