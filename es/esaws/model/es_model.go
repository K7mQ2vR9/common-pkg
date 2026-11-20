package model

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/K7mQ2vR9/base-pkg/commonTool"
	"github.com/K7mQ2vR9/base-pkg/es/esaws"
	"github.com/K7mQ2vR9/base-pkg/es/esaws/core"
	"github.com/opensearch-project/opensearch-go/opensearchapi"
)

// EsModel 将当前请求上下文与共享 OpenSearch 客户端组合；每个请求创建自己的模型实例。
type EsModel struct {
	// Ctx 为当前请求上下文，所有操作继承其取消、截止时间和链路。
	Ctx context.Context
	// Db 为服务级共享客户端，模型不负责关闭它。
	Db *core.Es
	// BusinessCode 非空时在模型索引名后追加下划线和业务码；不会自动创建该索引。
	BusinessCode string
}

// NewEsModel 创建请求级模型，不建立连接；nil 上下文或客户端在执行操作时返回错误。
func NewEsModel(ctx context.Context, db *core.Es, businessCode string) *EsModel {
	return &EsModel{Ctx: ctx, Db: db, BusinessCode: businessCode}
}

// GetDb 返回共享客户端，nil 模型或未设置客户端时返回 nil。
func (m *EsModel) GetDb() *core.Es {
	if m == nil {
		return nil
	}
	return m.Db
}

func (m *EsModel) begin() (context.Context, context.CancelFunc, error) {
	if m == nil || m.Db == nil {
		return nil, nil, errors.New("opensearch model client is required")
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

func (m *EsModel) indexName(value any) (string, error) {
	if missing(value) {
		return "", errors.New("opensearch index target is required")
	}
	table, ok := value.(esaws.IndexTable)
	if !ok || table.IndexName() == "" {
		return "", errors.New("opensearch non-empty IndexTable is required")
	}
	name := table.IndexName()
	if m.BusinessCode != "" {
		name += "_" + m.BusinessCode
	}
	return name, nil
}

// documentID 保留业务 ID；仅创建文档允许生成 UUIDv7，生成失败在网络写入前返回并保留错误链。
func documentID(value any, generate bool) (string, error) {
	if missing(value) {
		return "", errors.New("opensearch document is required")
	}
	schema, ok := value.(esaws.Schema)
	if !ok {
		return "", errors.New("opensearch Schema is required")
	}
	id := schema.GetId()
	if id == "" && generate {
		generatedID, err := commonTool.NewUUidV7()
		if err != nil {
			return "", fmt.Errorf("opensearch generate document ID: %w", err)
		}
		id = generatedID
	}
	if id == "" {
		return "", errors.New("opensearch document ID is required")
	}
	return id, nil
}

// closeOnError 清理 SDK 同时返回响应和错误时的正文；正常响应由 core 解析函数接管。
func closeOnError(response *opensearchapi.Response, err error) error {
	if err != nil && response != nil && response.Body != nil {
		_ = response.Body.Close()
	}
	return err
}

// IndexAuto 创建索引，已有同名索引按服务端错误返回，不自动删除或重建。
// index 是完整索引名称，本方法不会自动追加 BusinessCode 后缀。
func (m *EsModel) IndexAuto(index string) error {
	ctx, cancel, err := m.begin()
	if err != nil {
		return err
	}
	defer cancel()
	if index == "" {
		return errors.New("opensearch index is required")
	}
	response, err := m.Db.Indices.Create(index, m.Db.Indices.Create.WithContext(ctx))
	if err = closeOnError(response, err); err != nil {
		return err
	}
	return core.CheckResponse(ctx, response)
}

// InsertSchema 使用 create 语义写入，实现 IndexTable/Schema；空 ID 生成 UUIDv7，已有文档返回 409。
// UUID 生成失败时直接返回错误，不发送写入请求。
// 需要重发幂等时应预先设置稳定 ID；自动生成的 ID 不写回调用方对象。
func (m *EsModel) InsertSchema(data any) error {
	ctx, cancel, err := m.begin()
	if err != nil {
		return err
	}
	defer cancel()
	index, err := m.indexName(data)
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
	index, err := m.indexName(target)
	if err != nil {
		return err
	}
	if id == "" {
		return errors.New("opensearch document ID is required")
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
	index, err := m.indexName(target)
	if err != nil {
		return err
	}
	if id == "" {
		return errors.New("opensearch document ID is required")
	}
	response, err := m.Db.Delete(index, id, m.Db.Delete.WithContext(ctx))
	if err = closeOnError(response, err); err != nil {
		return err
	}
	return core.CheckResponse(ctx, response)
}

// UpdateSchema 使用 OpenSearch 的 doc 包装执行局部更新，文档不存在时返回 core.ErrNotFound。
func (m *EsModel) UpdateSchema(data any) error {
	ctx, cancel, err := m.begin()
	if err != nil {
		return err
	}
	defer cancel()
	index, err := m.indexName(data)
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

// Search 将命中正文解码到 target 切片指针，返回准确总数、原始 JSON 聚合结果和错误，不暴露已关闭的原始响应。
// indexTarget 提供 IndexName；固定 track_total_hits=true，空命中时目标为非 nil 空切片。
func (m *EsModel) Search(indexTarget any, target any, query map[string]any) (int64, core.Aggregations, error) {
	ctx, cancel, err := m.begin()
	if err != nil {
		return 0, nil, err
	}
	defer cancel()
	index, err := m.indexName(indexTarget)
	if err != nil {
		return 0, nil, err
	}
	body, err := json.Marshal(query)
	if err != nil {
		return 0, nil, err
	}
	response, err := m.Db.Search(m.Db.Search.WithContext(ctx), m.Db.Search.WithIndex(index), m.Db.Search.WithBody(bytes.NewReader(body)), m.Db.Search.WithTrackTotalHits(true))
	if err = closeOnError(response, err); err != nil {
		return 0, nil, err
	}
	return core.ParseSearchResponse(ctx, target, response)
}

// IndexSchema 全量覆盖指定 ID 的文档，不存在时创建；必须提供稳定 ID，保留原 refresh=true 行为。
// 字段缺失会被覆盖移除，局部修改使用 UpdateSchema；refresh=true 会增加写入成本。
func (m *EsModel) IndexSchema(data any) error {
	ctx, cancel, err := m.begin()
	if err != nil {
		return err
	}
	defer cancel()
	index, err := m.indexName(data)
	if err != nil {
		return err
	}
	id, err := documentID(data, false)
	if err != nil {
		return err
	}
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}
	response, err := m.Db.Index(index, bytes.NewReader(body), m.Db.Index.WithContext(ctx), m.Db.Index.WithDocumentID(id), m.Db.Index.WithRefresh("true"))
	if err = closeOnError(response, err); err != nil {
		return err
	}
	return core.CheckResponse(ctx, response)
}
