package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
)

var Separator = "." //nolint:gochecknoglobals

type MKey string

func NewMKey(parts ...string) MKey {
	return MKey(strings.Join(parts, Separator))
}

func (key MKey) Split() []string {
	return strings.Split(string(key), Separator)
}

type counter struct{ val uint64 }

func (c *counter) Increment() {
	atomic.AddUint64(&c.val, 1)
}

func (c *counter) Val() uint64 {
	return atomic.LoadUint64(&c.val)
}

type SafeMetrics struct {
	mutex       sync.RWMutex
	Data        map[MKey]*counter `json:"data"`
	PrettyPrint bool              `json:"-"`

	// ctx       context.Context
	bus       chan MKey
	busClosed bool
}

func (*SafeMetrics) New() *SafeMetrics {
	m := SafeMetrics{}
	m.mutex = sync.RWMutex{}
	m.Data = make(map[MKey]*counter)
	m.bus = make(chan MKey, 16)

	return &m
}

func (m *SafeMetrics) Add(name MKey) {
	if m.busClosed {
		return
	}

	m.bus <- name
}

func (m *SafeMetrics) Value(name MKey) uint64 {
	return m.Data[name].Val()
}

func (m *SafeMetrics) Collect(ctx context.Context) {
	for {
		select {
		case name := <-m.bus:
			m.mutex.Lock()
			if _, ok := m.Data[name]; !ok {
				m.Data[name] = &counter{}
			}
			m.Data[name].Increment()
			m.mutex.Unlock()
		case <-ctx.Done():
			m.busClosed = true
			// close(m.bus)
			return
		}
	}

}

func (m *SafeMetrics) MarshalJSON() ([]byte, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	data := map[MKey]uint64{}
	for mKey := range m.Data {
		data[mKey] = m.Value(mKey)
	}

	if !m.PrettyPrint {
		return json.Marshal(data)
	}

	res := make(map[string]interface{})
	nodes := m.parseNodes(data)
	for key, n := range nodes {
		res[key] = n.toJSON()
	}

	return json.Marshal(res)
}

func (m *SafeMetrics) WriteToFile(pathPrefix string, addTime bool) error {
	raw, err := m.MarshalJSON()
	if err != nil {
		return errors.Wrap(err, "can't marshal metrics")
	}

	var name = pathPrefix + "metrics.json"
	if addTime {
		name = fmt.Sprintf("%smetrics.%d.json", pathPrefix, time.Now().Unix())
	}

	return ioutil.WriteFile(name, raw, 0600)
}

func (m *SafeMetrics) parseNodes(data map[MKey]uint64) map[string]node {
	result := make(map[string]node)
	for mKey, count := range data {
		mKeyParts := mKey.Split()
		topName := mKeyParts[0]
		t := result[topName]
		t.name = topName
		result[topName] = *buildMetricsTree(&t, mKeyParts, count)
	}
	return result
}

type node struct {
	name     string
	level    int
	value    *uint64
	children map[string]*node
}

func (mn *node) toJSON() interface{} {
	res := make(map[string]interface{})
	if mn.children == nil {
		return mn.value
	}

	if mn.value != nil {
		res["value"] = *mn.value
	}

	for _, value := range mn.children {
		res[value.name] = value.toJSON()
	}

	return res
}

func buildMetricsTree(parent *node, mKeyParts []string, value uint64) *node {
	parent.name = mKeyParts[parent.level]
	if parent.level+1 > len(mKeyParts) {
		parent.value = &value
		return nil
	}

	if parent.level+1 == len(mKeyParts) {
		parent.value = &value
		return parent
	}

	child := &node{
		name:  mKeyParts[parent.level+1],
		level: parent.level + 1,
	}

	child = buildMetricsTree(child, mKeyParts, value)
	if child == nil {
		return parent
	}

	if parent.children == nil {
		parent.children = map[string]*node{}
	}

	exChild, ok := parent.children[child.name]
	if ok {
		if exChild.children == nil {
			exChild.children = map[string]*node{}
		}
		for key, value := range child.children {
			exChild.children[key] = value
		}
		child = exChild
	}

	parent.children[child.name] = child
	return parent
}
