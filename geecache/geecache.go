package geecache

import (
	"fmt"
	"log"
	"sync"

	"Go_Code/http-server/geecache/singleflight"
	pb "geecache/geecachepb"
)

// 定义一个接口
type Getter interface {
	Get(key string) ([]byte, error)
}

// 定义一个函数类型
type GetterFunc func(key string) ([]byte, error)

// 实现Getter接口的Get方法
func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key)
}

// GetterFunc 类型的实例被传递给另一个函数或方法，并在特定事件发生时被调用时，它就成为了一个回调函数。
func TestGetter() {
	var f GetterFunc = GetterFunc(func(key string) ([]byte, error) {
		return []byte(key), nil
	})

	expect := []byte("key")
	if v, _ := f.Get("key"); string(v) != string(expect) {
		println("Test failed")
	}
}

// Group是缓存的命名空间，每个Group拥有一个唯一的名称name
type Group struct {
	name      string
	getter    Getter //缓存未命中时获取源数据的回调(callback)
	mainCache cache
	peers     PeerPicker //选择节点
	loader    *singleflight.Group
}

var (
	mu     sync.RWMutex
	groups = make(map[string]*Group)
)

// 创建一个新的Group实例
func NewGroup(name string, cacheBytes int64, getter Getter) *Group {
	if getter == nil {
		panic("nil Getter")
	}
	mu.Lock()
	defer mu.Unlock()
	g := &Group{
		name:      name,
		getter:    getter,
		mainCache: cache{cacheBytes: cacheBytes},
		loader:    &singleflight.Group{},
	}
	groups[name] = g
	return g
}

// 获取指定名称的Group
func GetGroup(name string) *Group {
	mu.RLock()
	g := groups[name]
	mu.RUnlock()
	return g
}

func (g *Group) Get(key string) (ByteView, error) {
	if key == "" {
		return ByteView{}, fmt.Errorf("key is required") //返回一个空的ByteView结构体
	}
	if v, ok := g.mainCache.get(key); ok {
		log.Println("[GeeCache] hit") //缓存数据命中
		return v, nil
	}

	return g.load(key)
}

func (g *Group) load(key string) (value ByteView, err error) {
	//如果是分布式节点
	//使用singleflight.Group确保针对相同的key，无论调用多少次Do，回调函数fn都只会被调用一次
	viewi, err := g.loader.Do(key, func() (interface{}, error) {
		if g.peers != nil {
			if peer, ok := g.peers.PickPeer(key); ok {
				if value, err = g.getFromPeer(peer, key); err == nil {
					return value, nil
				}
				log.Println("[GeeCache] Failed to get from peer", err)
			}
		}

		return g.getLocally(key)
	})

	if err == nil {
		return viewi.(ByteView), nil
	}
	return
}

// 从本地节点获取数据
func (g *Group) getLocally(key string) (ByteView, error) {
	bytes, err := g.getter.Get(key)
	if err != nil {
		return ByteView{}, err
	}
	value := ByteView{b: cloneBytes(bytes)}
	g.populateCache(key, value)
	return value, nil

}

// 从远程节点获取数据
func (g *Group) getFromPeer(peer PeerGetter, key string) (ByteView, error) {
	req := &pb.Request{
		Group: g.name,
		Key:   key,
	}
	res := &pb.Response{}
	err := peer.Get(req, res)
	if err != nil {
		return ByteView{}, err
	}
	return ByteView{b: res.Value}, nil
}

func (g *Group) populateCache(key string, value ByteView) {
	g.mainCache.add(key, value)
}

func (g *Group) RegisiterPeers(peers PeerPicker) {
	if g.peers != nil {
		panic("RegisterPeerPicker called more than once")
	}
	g.peers = peers
}
