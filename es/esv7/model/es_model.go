package model

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"

	"github.com/K7mQ2vR9/base-pkg/commonTool"
	"github.com/K7mQ2vR9/base-pkg/es/esv7"
	"github.com/K7mQ2vR9/base-pkg/es/esv7/core"
	"github.com/elastic/go-elasticsearch/v7/esapi"
)

// EsModel 将当前请求上下文与共享 Elasticsearch 7 客户端组合；每个请求创建自己的模型实例。
type EsModel struct {
	// Ctx 为当前请求上下文，所有操作继承其取消、截止时间和链路。
	Ctx context.Context
	// Db 为服务级共享客户端，模型不负责关闭它。
	Db *core.Es
}

// NewEsModel 创建请求级模型，不建立连接；nil 上下文或客户端在执行操作时返回错误。
func NewEsModel(ctx context.Context, db *core.Es) *EsModel { return &EsModel{Ctx: ctx, Db: db} }

// GetDb 返回共享客户端，nil 模型或未设置客户端时返回 nil。
func (m *EsModel) GetDb() *core.Es {
	if m == nil {
		return nil
	}
	return m.Db
}

func (m *EsModel) begin() (context.Context, context.CancelFunc, error) {
	if m == nil || m.Db == nil {
		return nil, nil, errors.New("elasticsearch7 model client is required")
	}
	return m.Db.OperationContext(m.Ctx)
}

func missing(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Interface, reflect.Func, reflect.Chan:
		return v.IsNil()
	}
	return false
}

func indexName(value any) (string, error) {
	if missing(value) {
		return "", errors.New("elasticsearch7 index target is required")
	}
	table, ok := value.(esv7.IndexTable)
	if !ok || table.IndexName() == "" {
		return "", errors.New("elasticsearch7 non-empty IndexTable is required")
	}
	return table.IndexName(), nil
}

func documentID(value any, generate bool) (string, error) {
	if missing(value) {
		return "", errors.New("elasticsearch7 document is required")
	}
	schema, ok := value.(esv7.Schema)
	if !ok {
		return "", errors.New("elasticsearch7 Schema is required")
	}
	id := schema.GetId()
	if id == "" && generate {
		id = commonTool.GenUUID().String()
	}
	if id == "" {
		return "", errors.New("elasticsearch7 document ID is required")
	}
	return id, nil
}

// closeOnError 清理 SDK 同时返回响应和错误时的正文；正常响应由 core 解析函数接管。
func closeOnError(response *esapi.Response, err error) error {
	if err != nil && response != nil && response.Body != nil {
		_ = response.Body.Close()
	}
	return err
}

// IndexAuto 创建索引，已有同名索引按服务端错误返回，不自动删除或重建。
func (m *EsModel) IndexAuto(index string) error {
	ctx, cancel, err := m.begin()
	if err != nil {
		return err
	}
	defer cancel()
	if index == "" {
		return errors.New("elasticsearch7 index is required")
	}
	response, err := m.Db.Indices.Create(index, m.Db.Indices.Create.WithContext(ctx))
	if err = closeOnError(response, err); err != nil {
		return err
	}
	return core.CheckResponse(ctx, response)
}

// InsertSchema 使用 create 语义写入，实现 IndexTable/Schema；空 ID 生成 UUID，已有文档返回 409。
// 需要重发幂等时应预先设置稳定 ID；自动生成的 ID 不写回调用方对象。
func (m *EsModel) InsertSchema(data any) error {
	ctx, cancel, err := m.begin()
	if err != nil {
		return err
	}
	defer cancel()
	index, err := indexName(data)
	if err != nil {
		return err
	}
	id, err := documentID(data, true)
	if err != nil {
		return err
	}
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}
	response, err := m.Db.Create(index, id, bytes.NewReader(body), m.Db.Create.WithContext(ctx))
	if err = closeOnError(response, err); err != nil {
		return err
	}
	return core.CheckResponse(ctx, response)
}

// FindOne 将文档解码到调用方的非 nil 指针并设置 Schema ID，不存在时返回 core.ErrNotFound。
func (m *EsModel) FindOne(id string, target any) error {
	ctx, cancel, err := m.begin()
	if err != nil {
		return err
	}
	defer cancel()
	index, err := indexName(target)
	if err != nil {
		return err
	}
	if id == "" {
		return errors.New("elasticsearch7 document ID is required")
	}
	response, err := m.Db.Get(index, id, m.Db.Get.WithContext(ctx))
	if err = closeOnError(response, err); err != nil {
		return err
	}
	return core.ParseGetResponse(ctx, target, response)
}

// Delete 根据目标的 IndexName 删除指定文档；404 返回 core.ErrNotFound，由业务决定是否忽略。
func (m *EsModel) Delete(id string, target any) error {
	ctx, cancel, err := m.begin()
	if err != nil {
		return err
	}
	defer cancel()
	index, err := indexName(target)
	if err != nil {
		return err
	}
	if id == "" {
		return errors.New("elasticsearch7 document ID is required")
	}
	response, err := m.Db.Delete(index, id, m.Db.Delete.WithContext(ctx))
	if err = closeOnError(response, err); err != nil {
		return err
	}
	return core.CheckResponse(ctx, response)
}

// UpdateSchema 使用 Elasticsearch 的 doc 包装执行局部更新，文档不存在时返回 core.ErrNotFound。
func (m *EsModel) UpdateSchema(data any) error {
	ctx, cancel, err := m.begin()
	if err != nil {
		return err
	}
	defer cancel()
	index, err := indexName(data)
	if err != nil {
		return err
	}
	id, err := documentID(data, false)
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]any{"doc": data})
	if err != nil {
		return err
	}
	response, err := m.Db.Update(index, id, bytes.NewReader(body), m.Db.Update.WithContext(ctx))
	if err = closeOnError(response, err); err != nil {
		return err
	}
	return core.CheckResponse(ctx, response)
}

// Search 将命中正文解码到 target 切片指针，只返回准确总数和错误，不暴露已关闭的原始响应。
// indexTarget 提供 IndexName；固定 track_total_hits=true，空命中时目标为非 nil 空切片。
func (m *EsModel) Search(indexTarget any, target any, query map[string]any) (int64, error) {
	ctx, cancel, err := m.begin()
	if err != nil {
		return 0, err
	}
	defer cancel()
	index, err := indexName(indexTarget)
	if err != nil {
		return 0, err
	}
	body, err := json.Marshal(query)
	if err != nil {
		return 0, err
	}
	response, err := m.Db.Search(m.Db.Search.WithContext(ctx), m.Db.Search.WithIndex(index), m.Db.Search.WithBody(bytes.NewReader(body)), m.Db.Search.WithTrackTotalHits(true))
	if err = closeOnError(response, err); err != nil {
		return 0, err
	}
	return core.ParseSearchResponse(ctx, target, response)
}
