// Copyright (C) 2018. See AUTHORS.

package files

import (
	"bytes"
	"context"
	"encoding/binary"
	"io/ioutil"
	"os"
	"testing"

	"github.com/zeebo/assert"
	"github.com/zeebo/errs"
)

// newTestMetric constructs a temporary metric.
func newTestMetric(t testing.TB) (m *metric, cleanup func()) {
	t.Helper()

	dir, err := ioutil.TempDir("", "metric-")
	assert.NoError(t, err)

	// t.Log("temp dir:", dir)

	fch := newFileCache(fileCacheOptions{
		Handles: 100,
		Size:    1024,
		Cap:     10,
	})

	opts := metricOptions{
		fch:  fch,
		dir:  dir,
		name: "test.metric",
		max:  10,
	}

	m, err = newMetric(ctx, opts)
	assert.NoError(t, err)

	return m, func() {
		fch.Close()
		os.RemoveAll(dir)
	}
}

func TestMetric(t *testing.T) {
	t.Run("Write", func(t *testing.T) {
		m, cleanup := newTestMetric(t)
		defer cleanup()

		// test that a write that is too large cannot pass as the first write
		written, err := m.Write(ctx, 100, 200, make([]byte, 1024*1024))
		assert.Error(t, err)
		assert.That(t, !written)

		// test that a normal write works
		written, err = m.Write(ctx, 10, 20, make([]byte, 10))
		assert.NoError(t, err)
		assert.That(t, written)

		// test that a chronologically previous write does not work
		written, err = m.Write(ctx, 0, 10, make([]byte, 10))
		assert.NoError(t, err)
		assert.That(t, !written)

		// test that a write that is too large cannot pass after a valid write
		written, err = m.Write(ctx, 100, 200, make([]byte, 1024*1024))
		assert.Error(t, err)
		assert.That(t, !written)

		// test a write that would span multiple records
		written, err = m.Write(ctx, 100, 200, make([]byte, 4*1024))
		assert.NoError(t, err)
		assert.That(t, written)

		// assert.NoError(t, m.dump(ctx, os.Stdout))
	})

	t.Run("Read", func(t *testing.T) {
		t.Run("Read Only", func(t *testing.T) {
			dir, err := ioutil.TempDir("", "metric-")
			assert.NoError(t, err)
			defer os.RemoveAll(dir)

			_, err = newMetric(ctx, metricOptions{
				ro: true,
			})
			assert.That(t, os.IsNotExist(errs.Unwrap(err)))
		})

		test := func(t *testing.T, buf_size int) {
			m, cleanup := newTestMetric(t)
			defer cleanup()

			for i := int64(0); i < 1000; i++ {
				buf := make([]byte, buf_size)
				binary.BigEndian.PutUint64(buf, uint64(i))

				written, err := m.Write(ctx, i, i+1, buf)
				assert.NoError(t, err)
				assert.That(t, written)
			}

			m.Read(ctx, 10000, nil,
				func(ctx context.Context, start, end int64, data []byte) (
					bool, error) {

					buf := make([]byte, buf_size)
					binary.BigEndian.PutUint64(buf, uint64(start))
					assert.That(t, bytes.Equal(data, buf))
					return true, nil
				})
		}

		t.Run("Small", func(t *testing.T) { test(t, 512) })
		t.Run("Large", func(t *testing.T) { test(t, 4096) })

		t.Run("Empty", func(t *testing.T) {
			m, cleanup := newTestMetric(t)
			defer cleanup()

			err := m.Read(ctx, 1000, nil,
				func(ctx context.Context, _, _ int64, _ []byte) (
					bool, error) {

					assert.That(t, false)
					return true, nil
				})
			assert.NoError(t, err)
		})

		t.Run("Exhaustive", func(t *testing.T) {
			m, cleanup := newTestMetric(t)
			defer cleanup()

			for i := int64(0); i < 1000; i++ {
				written, err := m.Write(ctx, 50*i, 50*i+20, make([]byte, 10))
				assert.NoError(t, err)
				assert.That(t, written)
			}

			// 890 because we can keep up to 110 records as there are 10 per
			// file and 10 files, and we have 1 file of staging data.
			// everything before the earliest record should be empty.
			for i := int64(-100); i < 890; i++ {
				m.Read(ctx, 50*i, nil,
					func(ctx context.Context, _, _ int64, _ []byte) (
						bool, error) {

						assert.That(t, false)
						return true, nil
					})
			}

			// check right on the boundary and somewhere between records.
			for _, offset := range []int64{0, 10} {
				for i := int64(890); i < 1000; i++ {
					end := 50*i + offset
					first := true
					m.Read(ctx, 50*i+offset, nil,
						func(ctx context.Context, _, rec_end int64, _ []byte) (
							bool, error) {

							assert.That(t, rec_end < end)
							if first {
								assert.That(t, end-rec_end <= 40)
							}
							first = false
							return true, nil
						})
				}
			}

			// everything after the last record should be the last record
			for i := int64(1000); i < 1100; i++ {
				m.Read(ctx, 50*i, nil,
					func(ctx context.Context, _, end int64, _ []byte) (
						bool, error) {

						assert.Equal(t, end, int64(49970))
						return false, nil
					})
			}
		})
	})

	t.Run("ReadLast", func(t *testing.T) {
		test := func(t *testing.T, buf_size int) {
			m, cleanup := newTestMetric(t)
			defer cleanup()

			for i := int64(0); i < 1000; i++ {
				buf := make([]byte, buf_size)
				binary.BigEndian.PutUint64(buf, uint64(i))

				written, err := m.Write(ctx, i, i+1, buf)
				assert.NoError(t, err)
				assert.That(t, written)

				start, end, data, err := m.ReadLast(ctx, nil)
				assert.NoError(t, err)
				assert.Equal(t, start, i)
				assert.Equal(t, end, i+1)
				assert.That(t, bytes.Equal(data, buf))
			}
		}

		t.Run("Small", func(t *testing.T) { test(t, 512) })
		t.Run("Large", func(t *testing.T) { test(t, 4096) })

		t.Run("Empty", func(t *testing.T) {
			m, cleanup := newTestMetric(t)
			defer cleanup()

			start, end, data, err := m.ReadLast(ctx, nil)
			assert.NoError(t, err)
			assert.Equal(t, start, int64(0))
			assert.Equal(t, end, int64(0))
			assert.Nil(t, data)
		})
	})
}

func BenchmarkMetric(b *testing.B) {
	b.Run("Write", func(b *testing.B) {
		m, cleanup := newTestMetric(b)
		defer cleanup()

		data := make([]byte, 100)

		b.ReportAllocs()
		defer b.StopTimer()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			m.Write(ctx, int64(i), int64(i+1), data)
		}
	})

	b.Run("Read", func(b *testing.B) {
		test := func(b *testing.B, buf_size int) {
			m, cleanup := newTestMetric(b)
			defer cleanup()

			for i := int64(0); i < 1000; i++ {
				buf := make([]byte, buf_size)
				binary.BigEndian.PutUint64(buf, uint64(i))

				written, err := m.Write(ctx, i, i+1, buf)
				assert.NoError(b, err)
				assert.That(b, written)
			}

			buf := make([]byte, buf_size)
			size := 0
			m.Read(ctx, 10000, buf,
				func(ctx context.Context, start, end int64, data []byte) (
					bool, error) {

					size += len(data)
					return true, nil
				})

			b.SetBytes(int64(size))
			b.ReportAllocs()
			defer b.StopTimer()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				m.Read(ctx, 10000, buf,
					func(ctx context.Context, start, end int64, data []byte) (
						bool, error) {

						return true, nil
					})
			}
		}

		b.Run("Small", func(b *testing.B) { test(b, 512) })
		b.Run("Large", func(b *testing.B) { test(b, 4096) })
	})

	b.Run("ReadLast", func(b *testing.B) {
		test := func(b *testing.B, buf_size int) {
			m, cleanup := newTestMetric(b)
			defer cleanup()

			buf := make([]byte, buf_size)
			written, err := m.Write(ctx, 0, 1, buf)
			assert.NoError(b, err)
			assert.That(b, written)

			b.SetBytes(int64(buf_size))
			b.ReportAllocs()
			defer b.StopTimer()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				m.ReadLast(ctx, buf[:0])
			}

			b.StopTimer()
		}

		b.Run("Small", func(b *testing.B) { test(b, 512) })
		b.Run("Large", func(b *testing.B) { test(b, 4096) })
	})
}
