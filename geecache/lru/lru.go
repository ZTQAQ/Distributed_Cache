package lru

import (
	"container/list" // 双向链表
)

type EvictionPolicy int

const (
	LRU EvictionPolicy = iota
	LFU
)

type Cache struct {
	maxBytes     int64                         // 允许使用的最大内存
	nbytes       int64                         // 当前已使用的内存
	ll           *list.List                    // 双向链表
	cache        map[string]*list.Element      // 键是字符串，值是双向链表中对应节点的指针
	OnEvicted    func(key string, value Value) // 某条记录被移除时的回调函数
	evictionType EvictionPolicy                // 驱逐策略
	freqMap      map[string]int                // 记录元素的访问频率
}

// 双向链表节点的数据类型
type entry struct {
	key   string
	value Value
}

type Value interface {
	Len() int
}

// 实例化 Cache 结构
func New(maxBytes int64, evictionType EvictionPolicy, onEvicted func(string, Value)) *Cache {
	return &Cache{
		maxBytes:     maxBytes,
		ll:           list.New(),
		cache:        make(map[string]*list.Element),
		OnEvicted:    onEvicted,
		evictionType: evictionType,
		freqMap:      make(map[string]int),
	}
}

// 查找功能
func (c *Cache) Get(key string) (value Value, ok bool) {
	if ele, ok := c.cache[key]; ok {
		c.updateFrequency(key) // 更新访问频率
		c.ll.MoveToFront(ele)
		kv := ele.Value.(*entry)
		return kv.value, true
	}
	return
}

// 删除,把要删除的元素放到onEvicted函数中
func (c *Cache) RemoveOldest() {
	ele := c.ll.Back() //返回了链表的最后一个元素
	if ele != nil {
		c.ll.Remove(ele)
		kv := ele.Value.(*entry)
		delete(c.cache, kv.key) //从字典map中删除key和它映射的值（节点）
		c.nbytes -= int64(len(kv.key)) + int64(kv.value.Len())
		delete(c.freqMap, kv.key) // 从频率映射中删除
		if c.OnEvicted != nil {
			c.OnEvicted(kv.key, kv.value)
		}
	}
}

// 新增,ele是list.element(是一个结构体）存放的是next,prev,value,value存放的是entry结构体
func (c *Cache) Add(key string, value Value) {
	if ele, ok := c.cache[key]; ok { //修改
		c.ll.MoveToFront(ele)
		kv := ele.Value.(*entry)
		c.nbytes += int64(value.Len()) - int64(kv.value.Len())
		kv.value = value
	} else {
		ele := c.ll.PushFront(&entry{key, value}) //新增
		c.cache[key] = ele
		c.nbytes += int64(len(key)) + int64(value.Len())
		c.updateFrequency(key) // 新增元素，更新访问频率
	}
	for c.maxBytes != 0 && c.maxBytes < c.nbytes {
		c.evict() // 根据策略进行驱逐
	}
}

// 获取链表元素个数
func (c *Cache) Length() int {
	return c.ll.Len()
}

// 更新访问频率
func (c *Cache) updateFrequency(key string) {
	if _, ok := c.freqMap[key]; ok {
		c.freqMap[key]++
	} else {
		c.freqMap[key] = 1
	}
}

// 根据策略进行驱逐
func (c *Cache) evict() {
	switch c.evictionType {
	case LRU:
		c.RemoveOldest()
	case LFU:
		c.removeLFU()
	}
}

// 根据LFU策略进行驱逐
func (c *Cache) removeLFU() {
	var leastFreqKey string
	minFreq := int(^uint(0) >> 1) // set minFreq to maximum possible integer value

	for key, freq := range c.freqMap {
		if freq < minFreq {
			minFreq = freq
			leastFreqKey = key
		}
	}

	if leastFreqKey != "" {
		if ele, ok := c.cache[leastFreqKey]; ok {
			c.ll.Remove(ele)
			kv := ele.Value.(*entry)
			delete(c.cache, kv.key)
			delete(c.freqMap, kv.key)
			c.nbytes -= int64(len(kv.key)) + int64(kv.value.Len())
			if c.OnEvicted != nil {
				c.OnEvicted(kv.key, kv.value)
			}
		}
	}
}
