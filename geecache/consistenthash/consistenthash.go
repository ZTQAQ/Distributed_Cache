package consistenthash

import (
	"hash/crc32"
	"sort"
	"strconv"
)

type Hash func(data []byte) uint32

type Map struct {
	hash     Hash
	replicas int            //虚拟节点倍数
	keys     []int          //哈希环，储存所有节点的哈希值
	hashMap  map[int]string //虚拟节点与真实节点的映射表
}

func New(replicas int, fn Hash) *Map {
	m := &Map{
		replicas: replicas,
		hash:     fn,
		hashMap:  make(map[int]string),
	}
	if m.hash == nil {
		m.hash = crc32.ChecksumIEEE //默认的哈希函数
	}
	return m
}

// Add 方法允许传入 0 或多个真实节点的名称。
func (m *Map) Add(keys ...string) {
	for _, key := range keys {
		// 每个真实节点key生成m.replicas倍的虚拟节点
		for i := 0; i < m.replicas; i++ {
			//hash计算虚拟节点的哈希值
			hash := int(m.hash([]byte(strconv.Itoa(i) + key)))
			m.keys = append(m.keys, hash)
			m.hashMap[hash] = key //虚拟节点与真实节点的映射关系
		}
	}
	sort.Ints(m.keys)
}

// Get 方法根据给定的对象获取最靠近它的那个节点的名称
func (m *Map) Get(key string) string {
	if len(m.keys) == 0 {
		return ""
	}

	hash := int(m.hash([]byte(key)))
	//idx是第一个大于等于hash的元素的下标
	idx := sort.Search(len(m.keys), func(i int) bool {
		return m.keys[i] >= hash
	})

	return m.hashMap[m.keys[idx%len(m.keys)]]
}
