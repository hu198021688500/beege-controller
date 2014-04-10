package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

type KeyValue []string

func (this *KeyValue) Get(key string) (value string) {
	// FIXME: use Map()
	for _, kv := range *this {
		if strings.Index(kv, "=") == -1 {
			continue
		}
		parts := strings.SplitN(kv, "=", 2)
		if parts[0] != key {
			continue
		}
		if len(parts) < 2 {
			value = ""
		} else {
			value = parts[1]
		}
	}
	return
}

func (this *KeyValue) Exists(key string) bool {
	_, exists := this.Map()[key]
	return exists
}

func (this *KeyValue) Init(src *KeyValue) {
	(*this) = make([]string, 0, len(*src))
	for _, val := range *src {
		(*this) = append((*this), val)
	}
}

func (this *KeyValue) GetBool(key string) (value bool) {
	s := strings.ToLower(strings.Trim(this.Get(key), " \t"))
	if s == "" || s == "0" || s == "no" || s == "false" || s == "none" {
		return false
	}
	return true
}

func (this *KeyValue) SetBool(key string, value bool) {
	if value {
		this.Set(key, "1")
	} else {
		this.Set(key, "0")
	}
}

func (this *KeyValue) GetInt(key string) int {
	return int(this.GetInt64(key))
}

func (this *KeyValue) GetInt64(key string) int64 {
	s := strings.Trim(this.Get(key), " \t")
	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return val
}

func (this *KeyValue) SetInt(key string, value int) {
	this.Set(key, fmt.Sprintf("%d", value))
}

func (this *KeyValue) SetInt64(key string, value int64) {
	this.Set(key, fmt.Sprintf("%d", value))
}

// Returns nil if key not found
func (this *KeyValue) GetList(key string) []string {
	sval := this.Get(key)
	if sval == "" {
		return nil
	}
	l := make([]string, 0, 1)
	if err := json.Unmarshal([]byte(sval), &l); err != nil {
		l = append(l, sval)
	}
	return l
}

func (this *KeyValue) GetSubEnv(key string) *KeyValue {
	sval := this.Get(key)
	if sval == "" {
		return nil
	}
	buf := bytes.NewBufferString(sval)
	var sub KeyValue
	if err := sub.Decode(buf); err != nil {
		return nil
	}
	return &sub
}

func (this *KeyValue) SetSubEnv(key string, sub *KeyValue) error {
	var buf bytes.Buffer
	if err := sub.Encode(&buf); err != nil {
		return err
	}
	this.Set(key, string(buf.Bytes()))
	return nil
}

func (this *KeyValue) GetJson(key string, iface interface{}) error {
	sval := this.Get(key)
	if sval == "" {
		return nil
	}
	return json.Unmarshal([]byte(sval), iface)
}

func (this *KeyValue) SetJson(key string, value interface{}) error {
	sval, err := json.Marshal(value)
	if err != nil {
		return err
	}
	this.Set(key, string(sval))
	return nil
}

func (this *KeyValue) SetList(key string, value []string) error {
	return this.SetJson(key, value)
}

func (this *KeyValue) Set(key, value string) {
	*this = append(*this, key+"="+value)
}

func NewDecoder(src io.Reader) *Decoder {
	return &Decoder{
		json.NewDecoder(src),
	}
}

type Decoder struct {
	*json.Decoder
}

func (decoder *Decoder) Decode() (*KeyValue, error) {
	m := make(map[string]interface{})
	if err := decoder.Decoder.Decode(&m); err != nil {
		return nil, err
	}
	this := &KeyValue{}
	for key, value := range m {
		this.SetAuto(key, value)
	}
	return this, nil
}

// DecodeEnv decodes `src` as a json dictionary, and adds
// each decoded key-value pair to the environment.
//
// If `src` cannot be decoded as a json dictionary, an error
// is returned.
func (this *KeyValue) Decode(src io.Reader) error {
	m := make(map[string]interface{})
	if err := json.NewDecoder(src).Decode(&m); err != nil {
		return err
	}
	for k, v := range m {
		this.SetAuto(k, v)
	}
	return nil
}

func (this *KeyValue) SetAuto(k string, v interface{}) {
	// FIXME: we fix-convert float values to int, because
	// encoding/json decodes integers to float64, but cannot encode them back.
	// (See http://golang.org/src/pkg/encoding/json/decode.go#L46)
	if fval, ok := v.(float64); ok {
		this.SetInt64(k, int64(fval))
	} else if sval, ok := v.(string); ok {
		this.Set(k, sval)
	} else if val, err := json.Marshal(v); err == nil {
		this.Set(k, string(val))
	} else {
		this.Set(k, fmt.Sprintf("%v", v))
	}
}

func (this *KeyValue) Encode(dst io.Writer) error {
	m := make(map[string]interface{})
	for k, v := range this.Map() {
		var val interface{}
		if err := json.Unmarshal([]byte(v), &val); err == nil {
			// FIXME: we fix-convert float values to int, because
			// encoding/json decodes integers to float64, but cannot encode them back.
			// (See http://golang.org/src/pkg/encoding/json/decode.go#L46)
			if fval, isFloat := val.(float64); isFloat {
				val = int(fval)
			}
			m[k] = val
		} else {
			m[k] = v
		}
	}
	if err := json.NewEncoder(dst).Encode(&m); err != nil {
		return err
	}
	return nil
}

func (this *KeyValue) WriteTo(dst io.Writer) (n int64, err error) {
	// FIXME: return the number of bytes written to respect io.WriterTo
	return 0, this.Encode(dst)
}

func (this *KeyValue) Import(src interface{}) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("ImportEnv: %s", err)
		}
	}()
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(src); err != nil {
		return err
	}
	if err := this.Decode(&buf); err != nil {
		return err
	}
	return nil
}

func (this *KeyValue) Map() map[string]string {
	m := make(map[string]string)
	for _, kv := range *this {
		parts := strings.SplitN(kv, "=", 2)
		m[parts[0]] = parts[1]
	}
	return m
}

type Table struct {
	Data    []*KeyValue
	sortKey string
	Chan    chan *KeyValue
}

func NewTable(sortKey string, sizeHint int) *Table {
	return &Table{
		make([]*KeyValue, 0, sizeHint),
		sortKey,
		make(chan *KeyValue),
	}
}

func (t *Table) SetKey(sortKey string) {
	t.sortKey = sortKey
}

func (t *Table) Add(this *KeyValue) {
	t.Data = append(t.Data, this)
}

func (t *Table) Len() int {
	return len(t.Data)
}

func (t *Table) Less(a, b int) bool {
	return t.lessBy(a, b, t.sortKey)
}

func (t *Table) lessBy(a, b int, by string) bool {
	keyA := t.Data[a].Get(by)
	keyB := t.Data[b].Get(by)
	intA, errA := strconv.ParseInt(keyA, 10, 64)
	intB, errB := strconv.ParseInt(keyB, 10, 64)
	if errA == nil && errB == nil {
		return intA < intB
	}
	return keyA < keyB
}

func (t *Table) Swap(a, b int) {
	tmp := t.Data[a]
	t.Data[a] = t.Data[b]
	t.Data[b] = tmp
}

func (t *Table) Sort() {
	sort.Sort(t)
}

func (t *Table) ReverseSort() {
	sort.Sort(sort.Reverse(t))
}

func (t *Table) WriteListTo(dst io.Writer) (n int64, err error) {
	if _, err := dst.Write([]byte{'['}); err != nil {
		return -1, err
	}
	n = 1
	for i, this := range t.Data {
		bytes, err := this.WriteTo(dst)
		if err != nil {
			return -1, err
		}
		n += bytes
		if i != len(t.Data)-1 {
			if _, err := dst.Write([]byte{','}); err != nil {
				return -1, err
			}
			n += 1
		}
	}
	if _, err := dst.Write([]byte{']'}); err != nil {
		return -1, err
	}
	return n + 1, nil
}

func (t *Table) ToListString() (string, error) {
	buffer := bytes.NewBuffer(nil)
	if _, err := t.WriteListTo(buffer); err != nil {
		return "", err
	}
	return buffer.String(), nil
}

func (t *Table) WriteTo(dst io.Writer) (n int64, err error) {
	for _, this := range t.Data {
		bytes, err := this.WriteTo(dst)
		if err != nil {
			return -1, err
		}
		n += bytes
	}
	return n, nil
}

func (t *Table) ReadListFrom(src []byte) (n int64, err error) {
	var array []interface{}

	if err := json.Unmarshal(src, &array); err != nil {
		return -1, err
	}

	for _, item := range array {
		if m, ok := item.(map[string]interface{}); ok {
			this := &KeyValue{}
			for key, value := range m {
				this.SetAuto(key, value)
			}
			t.Add(this)
		}
	}

	return int64(len(src)), nil
}

func (t *Table) ReadFrom(src io.Reader) (n int64, err error) {
	decoder := NewDecoder(src)
	for {
		this, err := decoder.Decode()
		if err == io.EOF {
			return 0, nil
		} else if err != nil {
			return -1, err
		}
		t.Add(this)
	}
	return 0, nil
}
