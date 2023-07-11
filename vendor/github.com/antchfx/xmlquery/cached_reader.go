package xmlquery

import (
	"bufio"
)

type cachedReader struct {
	buffer *bufio.Reader
	cache []byte
	cacheCap int
	cacheLen int
	caching bool
}

func newCachedReader(r *bufio.Reader) *cachedReader {
	return &cachedReader{
		buffer:   r,
		cache:    make([]byte, 4096),
		cacheCap: 4096,
		cacheLen: 0,
		caching:  false,
	}
}

func (c *cachedReader) StartCaching() {
	c.cacheLen = 0
	c.caching = true
}

func (c *cachedReader) ReadByte() (byte, error) {
	if !c.caching {
		return c.buffer.ReadByte()
	}
	b, err := c.buffer.ReadByte()
	if err != nil {
		return b, err
	}
	if c.cacheLen < c.cacheCap {
		c.cache[c.cacheLen] = b
		c.cacheLen++
	}
	return b, err
}

func (c *cachedReader) Cache() []byte {
	return c.cache[:c.cacheLen]
}

func (c *cachedReader) StopCaching() {
	c.caching = false
}

func (c *cachedReader) Read(p []byte) (int, error) {
	n, err := c.buffer.Read(p)
	if err != nil {
		return n, err
	}
	if c.caching && c.cacheLen < c.cacheCap {
		for i := 0; i < n; i++ {
			c.cache[c.cacheLen] = p[i]
			c.cacheLen++
			if c.cacheLen >= c.cacheCap {
				break
			}
		}
	}
	return n, err
}

