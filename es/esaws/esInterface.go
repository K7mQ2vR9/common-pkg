package esaws

// IndexTable 提供 OpenSearch 业务索引基础名；模型可追加 BusinessCode 后缀。
type IndexTable interface {
	IndexName() string
}

// Schema 提供 OpenSearch 文档 ID，FindOne 成功后将实际 ID 写入目标。
type Schema interface {
	GetId() string
	SetId(id string)
}
