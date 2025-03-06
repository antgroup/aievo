package db

import "time"

// Document 文档表
type Document struct {
	ID          int64     `gorm:"primaryKey;column:id;autoIncrement"`
	DocID       string    `gorm:"column:doc_id;type:varchar(64);not null;index"`
	Title       string    `gorm:"column:title;type:varchar(255);not null"`
	Content     string    `gorm:"column:content;type:text;not null"`
	TextUnitIDs string    `gorm:"column:text_unit_ids;type:text"`
	GmtCreate   time.Time `gorm:"column:gmt_create;autoCreateTime"`
	GmtModified time.Time `gorm:"column:gmt_modified;autoUpdateTime"`
}

// TextUnit 文本单元表
type TextUnit struct {
	ID              int64     `gorm:"primaryKey;column:id;autoIncrement"`
	UnitID          string    `gorm:"column:unit_id;type:varchar(64);not null;index"`
	Text            string    `gorm:"column:text;type:text;not null"`
	DocumentIDs     string    `gorm:"column:document_ids;type:text"`
	EntityIDs       string    `gorm:"column:entity_ids;type:text"`
	RelationshipIDs string    `gorm:"column:relationship_ids;type:text"`
	NumToken        int       `gorm:"column:num_token;not null;default:0"`
	GmtCreate       time.Time `gorm:"column:gmt_create;autoCreateTime"`
	GmtModified     time.Time `gorm:"column:gmt_modified;autoUpdateTime"`
}

// Entity 实体表
type Entity struct {
	ID          int64     `gorm:"primaryKey;column:id;autoIncrement"`
	EntityID    string    `gorm:"column:entity_id;type:varchar(64);not null;index"`
	Title       string    `gorm:"column:title;type:varchar(255);not null"`
	Type        string    `gorm:"column:type;type:varchar(64);not null"`
	Description string    `gorm:"column:description;type:text"`
	Degree      int       `gorm:"column:degree;not null;default:0"`
	Communities string    `gorm:"column:communities;type:text"`
	TextUnitIDs string    `gorm:"column:text_unit_ids;type:text"`
	GmtCreate   time.Time `gorm:"column:gmt_create;autoCreateTime"`
	GmtModified time.Time `gorm:"column:gmt_modified;autoUpdateTime"`
}

// Relationship 关系表
type Relationship struct {
	ID             int64     `gorm:"primaryKey;column:id;autoIncrement"`
	RelationshipID string    `gorm:"column:relationship_id;type:varchar(64);not null;index"`
	SourceEntityID string    `gorm:"column:source_entity_id;type:varchar(64);not null"`
	TargetEntityID string    `gorm:"column:target_entity_id;type:varchar(64);not null"`
	Description    string    `gorm:"column:description;type:text"`
	Weight         float64   `gorm:"column:weight;type:decimal(10,4);not null;default:0"`
	CombinedDegree int       `gorm:"column:combined_degree;not null;default:0"`
	TextUnitIDs    string    `gorm:"column:text_unit_ids;type:text"`
	GmtCreate      time.Time `gorm:"column:gmt_create;autoCreateTime"`
	GmtModified    time.Time `gorm:"column:gmt_modified;autoUpdateTime"`
}

// TmpRelationship 临时关系表
type TmpRelationship struct {
	ID                int64     `gorm:"primaryKey;column:id;autoIncrement"`
	TmpRelationshipID string    `gorm:"column:tmp_relationship_id;type:varchar(64);not null;index"`
	Source            string    `gorm:"column:source;type:varchar(255);not null"`
	Target            string    `gorm:"column:target;type:varchar(255);not null"`
	Description       string    `gorm:"column:description;type:text"`
	Weight            float64   `gorm:"column:weight;type:decimal(10,4);not null;default:0"`
	CombinedDegree    int       `gorm:"column:combined_degree;not null;default:0"`
	TextUnitIDs       string    `gorm:"column:text_unit_ids;type:text"`
	SourceID          string    `gorm:"column:source_id;type:varchar(64);not null"`
	GmtCreate         time.Time `gorm:"column:gmt_create;autoCreateTime"`
	GmtModified       time.Time `gorm:"column:gmt_modified;autoUpdateTime"`
}

// Node 节点表
type Node struct {
	ID          int64     `gorm:"primaryKey;column:id;autoIncrement"`
	NodeID      string    `gorm:"column:node_id;type:varchar(64);not null;index"`
	Title       string    `gorm:"column:title;type:varchar(255);not null"`
	Community   int       `gorm:"column:community;not null"`
	Level       int       `gorm:"column:level;not null"`
	Degree      int       `gorm:"column:degree;not null;default:0"`
	GmtCreate   time.Time `gorm:"column:gmt_create;autoCreateTime"`
	GmtModified time.Time `gorm:"column:gmt_modified;autoUpdateTime"`
}

// Community 社区表
type Community struct {
	ID              int64     `gorm:"primaryKey;column:id;autoIncrement"`
	CommunityID     string    `gorm:"column:community_id;type:varchar(64);not null;index"`
	Title           string    `gorm:"column:title;type:varchar(255);not null"`
	Community       int       `gorm:"column:community;not null"`
	Level           int       `gorm:"column:level;not null"`
	RelationshipIDs string    `gorm:"column:relationship_ids;type:text"`
	TextUnitIDs     string    `gorm:"column:text_unit_ids;type:text"`
	Parent          int       `gorm:"column:parent;not null;default:0"`
	EntityIDs       string    `gorm:"column:entity_ids;type:text"`
	Period          string    `gorm:"column:period;type:varchar(64)"`
	Size            int       `gorm:"column:size;not null;default:0"`
	GmtCreate       time.Time `gorm:"column:gmt_create;autoCreateTime"`
	GmtModified     time.Time `gorm:"column:gmt_modified;autoUpdateTime"`
}

// Report 报告表
type Report struct {
	ID                int64     `gorm:"primaryKey;column:id;autoIncrement"`
	Community         int       `gorm:"column:community;not null"`
	Title             string    `gorm:"column:title;type:varchar(255);not null"`
	Summary           string    `gorm:"column:summary;type:text"`
	Rating            float64   `gorm:"column:rating;type:decimal(10,4);not null;default:0"`
	RatingExplanation string    `gorm:"column:rating_explanation;type:text"`
	GmtCreate         time.Time `gorm:"column:gmt_create;autoCreateTime"`
	GmtModified       time.Time `gorm:"column:gmt_modified;autoUpdateTime"`
}

// Finding 报告发现表
type Finding struct {
	ID          int64     `gorm:"primaryKey;column:id;autoIncrement"`
	ReportID    int64     `gorm:"column:report_id;not null"`
	Summary     string    `gorm:"column:summary;type:text"`
	Explanation string    `gorm:"column:explanation;type:text"`
	GmtCreate   time.Time `gorm:"column:gmt_create;autoCreateTime"`
	GmtModified time.Time `gorm:"column:gmt_modified;autoUpdateTime"`
}

// Knowledge 知识汇总表
type Knowledge struct {
	ID                 int64     `gorm:"primaryKey;column:id;autoIncrement"`
	DocumentIDs        string    `gorm:"column:document_ids;type:text"`
	TextUnitIDs        string    `gorm:"column:textunit_ids;type:text"`
	RelationshipIDs    string    `gorm:"column:relationship_ids;type:text"`
	TmpRelationshipIDs string    `gorm:"column:tmprelationship_ids;type:text"`
	EntityIDs          string    `gorm:"column:entity_ids;type:text"`
	CommunityIDs       string    `gorm:"column:community_ids;type:text"`
	NodeIDs            string    `gorm:"column:node_ids;type:text"`
	ReportIDs          string    `gorm:"column:report_ids;type:text"`
	GmtCreate          time.Time `gorm:"column:gmt_create;autoCreateTime"`
	GmtModified        time.Time `gorm:"column:gmt_modified;autoUpdateTime"`
}

// TableName 为每个模型指定表名
func (Document) TableName() string {
	return "aievo_rag_document"
}

func (TextUnit) TableName() string {
	return "aievo_rag_textunit"
}

func (Entity) TableName() string {
	return "aievo_rag_entity"
}

func (Relationship) TableName() string {
	return "aievo_rag_relationship"
}

func (TmpRelationship) TableName() string {
	return "aievo_rag_tmprelationship"
}

func (Node) TableName() string {
	return "aievo_rag_node"
}

func (Community) TableName() string {
	return "aievo_rag_community"
}

func (Report) TableName() string {
	return "aievo_rag_report"
}

func (Finding) TableName() string {
	return "aievo_rag_finding"
}

func (Knowledge) TableName() string {
	return "aievo_rag_knowledge"
}
