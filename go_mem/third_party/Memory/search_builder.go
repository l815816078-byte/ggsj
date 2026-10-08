package Memory

import (
	"encoding/json"
	"errors"
)

type SearchValueType string

const (
	SearchTypeI8     SearchValueType = "int8_t"
	SearchTypeI16    SearchValueType = "int16_t"
	SearchTypeI32    SearchValueType = "int32_t"
	SearchTypeI64    SearchValueType = "int64_t"
	SearchTypeUI8    SearchValueType = "u_int8_t"
	SearchTypeUI16   SearchValueType = "u_int16_t"
	SearchTypeUI32   SearchValueType = "uint32_t"
	SearchTypeUI64   SearchValueType = "uint64_t"
	SearchTypeChar   SearchValueType = "char"
	SearchTypeUChar  SearchValueType = "unsigned char"
	SearchTypeF32    SearchValueType = "float"
	SearchTypeF64    SearchValueType = "double"
	SearchTypeString SearchValueType = "char*"
)

type SearchRegion string

const (
	RegionAll   SearchRegion = "all"
	RegionBad   SearchRegion = "bad"
	RegionVideo SearchRegion = "video"
	RegionCA    SearchRegion = "ca"
	RegionCB    SearchRegion = "cb"
	RegionCD    SearchRegion = "cd"
	RegionCH    SearchRegion = "ch"
	RegionJH    SearchRegion = "jh"
	RegionJ     SearchRegion = "j"
	RegionA     SearchRegion = "a"
	RegionXS    SearchRegion = "xs"
	RegionS     SearchRegion = "s"
	RegionAS    SearchRegion = "as"
	RegionOther SearchRegion = "other"
	RegionXA    SearchRegion = "xa"
	RegionPS    SearchRegion = "ps"
)

type SearchCompare int

const (
	SearchEqual SearchCompare = iota
	SearchLV
	SearchHV
)

type SearchClause struct {
	Type    SearchValueType
	Compare SearchCompare
	Value   any
}

type SearchDataFilter struct {
	Offset int
	Clause SearchClause
}

type SearchRange struct {
	Star uint64 `json:"Star"`
	End  uint64 `json:"End"`
}

type SearchRequest struct {
	PID    int                `json:"pid,omitempty"`
	Head   SearchClause       `json:"head"`
	Ranges []SearchRange      `json:"rangs,omitempty"`
	Types  []string           `json:"types,omitempty"`
	Data   []SearchDataFilter `json:"data,omitempty"`
	Remove []SearchRange      `json:"Remove,omitempty"`
	Nop    bool               `json:"nop,omitempty"`
	Max    uint64             `json:"max,omitempty"`
}

func EqualSearch(valueType SearchValueType, value any) SearchClause {
	return SearchClause{Type: valueType, Compare: SearchEqual, Value: value}
}

func LVSearch(valueType SearchValueType, value any) SearchClause {
	return SearchClause{Type: valueType, Compare: SearchLV, Value: value}
}

func HVSearch(valueType SearchValueType, value any) SearchClause {
	return SearchClause{Type: valueType, Compare: SearchHV, Value: value}
}

func EqualFilter(valueType SearchValueType, offset int, value any) SearchDataFilter {
	return SearchDataFilter{Offset: offset, Clause: EqualSearch(valueType, value)}
}

func LVFilter(valueType SearchValueType, offset int, value any) SearchDataFilter {
	return SearchDataFilter{Offset: offset, Clause: LVSearch(valueType, value)}
}

func HVFilter(valueType SearchValueType, offset int, value any) SearchDataFilter {
	return SearchDataFilter{Offset: offset, Clause: HVSearch(valueType, value)}
}

func MakeSearchRange(start uint64, end uint64) SearchRange {
	return SearchRange{Star: start, End: end}
}

func NewSearchRequest(head SearchClause) *SearchRequest {
	return &SearchRequest{Head: head}
}

func (c SearchCompare) key() (string, error) {
	switch c {
	case SearchEqual:
		return "data", nil
	case SearchLV:
		return "lv", nil
	case SearchHV:
		return "hv", nil
	default:
		return "", errors.New("invalid search compare")
	}
}

func (c SearchClause) MarshalJSON() ([]byte, error) {
	if c.Type == "" {
		return nil, errors.New("search clause type is empty")
	}
	if c.Value == nil {
		return nil, errors.New("search clause value is nil")
	}
	key, err := c.Compare.key()
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"type": string(c.Type),
		key:    c.Value,
	}
	return json.Marshal(payload)
}

func (f SearchDataFilter) MarshalJSON() ([]byte, error) {
	if f.Clause.Type == "" {
		return nil, errors.New("search filter type is empty")
	}
	if f.Clause.Value == nil {
		return nil, errors.New("search filter value is nil")
	}
	key, err := f.Clause.Compare.key()
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"type":   string(f.Clause.Type),
		"offset": f.Offset,
		key:      f.Clause.Value,
	}
	return json.Marshal(payload)
}

func (r *SearchRequest) WithPID(pid int) *SearchRequest {
	r.PID = pid
	return r
}

func (r *SearchRequest) WithRange(start uint64, end uint64) *SearchRequest {
	r.Ranges = append(r.Ranges, MakeSearchRange(start, end))
	return r
}

func (r *SearchRequest) WithRanges(ranges ...SearchRange) *SearchRequest {
	r.Ranges = append(r.Ranges, ranges...)
	return r
}

func (r *SearchRequest) WithRegions(regions ...SearchRegion) *SearchRequest {
	for _, region := range regions {
		r.Types = append(r.Types, string(region))
	}
	return r
}

func (r *SearchRequest) WithTypes(types ...string) *SearchRequest {
	r.Types = append(r.Types, types...)
	return r
}

func (r *SearchRequest) WithDataFilters(filters ...SearchDataFilter) *SearchRequest {
	r.Data = append(r.Data, filters...)
	return r
}

func (r *SearchRequest) WithRemoveRange(start uint64, end uint64) *SearchRequest {
	r.Remove = append(r.Remove, MakeSearchRange(start, end))
	return r
}

func (r *SearchRequest) WithRemoveRanges(ranges ...SearchRange) *SearchRequest {
	r.Remove = append(r.Remove, ranges...)
	return r
}

func (r *SearchRequest) WithNop(enabled bool) *SearchRequest {
	r.Nop = enabled
	return r
}

func (r *SearchRequest) WithMax(max uint64) *SearchRequest {
	r.Max = max
	return r
}

func (r *SearchRequest) JSON() (string, error) {
	raw, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func (c *Client) NewSearchRequest(head SearchClause) *SearchRequest {
	return NewSearchRequest(head).WithPID(c.pid)
}

func (c *Client) SearchWithRequest(request *SearchRequest) ([]uint64, error) {
	return c.searchRequest(request, false)
}

func (c *Client) SearchFastWithRequest(request *SearchRequest) ([]uint64, error) {
	return c.searchRequest(request, true)
}

func (c *Client) searchRequest(request *SearchRequest, fast bool) ([]uint64, error) {
	if request == nil {
		return nil, errors.New("search request is nil")
	}
	payload := *request
	if payload.PID <= 0 {
		payload.PID = c.pid
	}
	raw, err := payload.JSON()
	if err != nil {
		return nil, err
	}
	return c.searchRaw(raw, fast)
}
