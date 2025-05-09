package db

import (
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/antgroup/aievo/rag"
	"gorm.io/gorm"
)

type Storage struct {
	db *gorm.DB
}

func NewStorage(opts ...DBStorageOption) *Storage {
	s := &Storage{}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func serialize(wfCtx *rag.WorkflowContext, filePath string) error {
	type serializableContext struct {
		Id               int64
		IndexProgress    int
		Documents        []*rag.Document
		TextUnits        []*rag.TextUnit
		Relationships    []*rag.Relationship
		TmpRelationships []*rag.TmpRelationship
		Entities         []*rag.Entity
		Communities      []*rag.Community
		Nodes            []*rag.Node
		Reports          []*rag.Report
	}

	sc := &serializableContext{
		Id:               wfCtx.Id,
		IndexProgress:    wfCtx.IndexProgress,
		Documents:        wfCtx.Documents,
		TextUnits:        wfCtx.TextUnits,
		Relationships:    wfCtx.Relationships,
		TmpRelationships: wfCtx.TmpRelationships,
		Entities:         wfCtx.Entities,
		Communities:      wfCtx.Communities,
		Nodes:            wfCtx.Nodes,
		Reports:          wfCtx.Reports,
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}

	encoder := gob.NewEncoder(file)
	if err = encoder.Encode(sc); err != nil {
		_ = file.Close()
		_ = os.Remove(filePath)
		return err
	}

	_ = file.Close()
	return nil
}

func deserialize(filePath string, wfCtx *rag.WorkflowContext) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}

	decoder := gob.NewDecoder(file)
	if err = decoder.Decode(wfCtx); err != nil {
		_ = file.Close()
		_ = os.Remove(filePath)
		return err
	}

	_ = file.Close()

	return nil
}

func RefreshCache(wfCtx *rag.WorkflowContext) {
	cacheFilePath := fmt.Sprintf("%s/%s_%d", wfCtx.CacheDir, "rag_cache_", wfCtx.Id)
	_ = os.Remove(cacheFilePath)

	if wfCtx.Config.DB == nil {
		return
	}

	storage := NewStorage(WithDB(wfCtx.Config.DB))

	_ = storage.Load(context.Background(), wfCtx)
}

func (s *Storage) Load(ctx context.Context, wfCtx *rag.WorkflowContext) error {
	if wfCtx.Id == 0 {
		return errors.New("id is not set")
	}

	cacheFilePath := fmt.Sprintf("%s/%s_%d", wfCtx.CacheDir, "rag_cache_", wfCtx.Id)

	if err := deserialize(cacheFilePath, wfCtx); err == nil {
		return nil
	}

	if wfCtx.Documents == nil || wfCtx.TextUnits == nil || wfCtx.Entities == nil ||
		wfCtx.Relationships == nil || wfCtx.TmpRelationships == nil || wfCtx.Nodes == nil ||
		wfCtx.Communities == nil || wfCtx.Reports == nil {
		return errors.New("please call rag.NewWorkflowContext() to create a new workflow context")
	}

	var knowledge Knowledge
	if err := s.db.Where("id = ?", wfCtx.Id).First(&knowledge).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		} else {
			knowledge.ID = wfCtx.Id
			knowledge.IndexProgress = 0
		}
	}

	wfCtx.IndexProgress = knowledge.IndexProgress

	if err := s.loadDocuments(ctx, knowledge, wfCtx); err != nil {
		return err
	}

	if err := s.loadTextUnits(ctx, knowledge, wfCtx); err != nil {
		return err
	}

	if err := s.loadEntities(ctx, knowledge, wfCtx); err != nil {
		return err
	}

	if err := s.loadRelationships(ctx, knowledge, wfCtx); err != nil {
		return err
	}

	if err := s.loadTmpRelationships(ctx, knowledge, wfCtx); err != nil {
		return err
	}

	if err := s.loadNodes(ctx, knowledge, wfCtx); err != nil {
		return err
	}

	if err := s.loadCommunities(ctx, knowledge, wfCtx); err != nil {
		return err
	}

	if err := s.loadReports(ctx, knowledge, wfCtx); err != nil {
		return err
	}

	_ = serialize(wfCtx, cacheFilePath)

	return nil
}

func (s *Storage) Save(ctx context.Context, wfCtx *rag.WorkflowContext, indexProgress int) error {
	tx := s.db.Begin()
	defer tx.Commit()

	var knowledge Knowledge
	if err := tx.Where("id = ?", wfCtx.Id).First(&knowledge).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	} else {
		if err := s.deleteAllData(ctx, knowledge, tx); err != nil {
			tx.Rollback()
			return err
		}
	}

	knowledge.ID = wfCtx.Id
	knowledge.IndexProgress = indexProgress

	err := s.saveDocument(ctx, wfCtx, &knowledge, tx)
	if err != nil {
		tx.Rollback()
		return err
	}

	err = s.saveTextUnit(ctx, wfCtx, &knowledge, tx)
	if err != nil {
		tx.Rollback()
		return err
	}

	err = s.saveEntity(ctx, wfCtx, &knowledge, tx)
	if err != nil {
		tx.Rollback()
		return err
	}

	err = s.saveRelationship(ctx, wfCtx, &knowledge, tx)
	if err != nil {
		tx.Rollback()
		return err
	}

	err = s.saveTmpRelationship(ctx, wfCtx, &knowledge, tx)
	if err != nil {
		tx.Rollback()
		return err
	}

	err = s.saveNode(ctx, wfCtx, &knowledge, tx)
	if err != nil {
		tx.Rollback()
		return err
	}

	err = s.saveCommunity(ctx, wfCtx, &knowledge, tx)
	if err != nil {
		tx.Rollback()
		return err
	}

	err = s.saveReport(ctx, wfCtx, &knowledge, tx)
	if err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Save(&knowledge).Error; err != nil {
		tx.Rollback()
		return err
	}

	return nil
}

func int64ArrayToStr(int64s []int64) string {
	strs := make([]string, len(int64s))
	for i, v := range int64s {
		strs[i] = strconv.FormatInt(v, 10)
	}
	return strings.Join(strs, ",")
}

func (s *Storage) loadDocuments(_ context.Context, knowledge Knowledge, wfCtx *rag.WorkflowContext) error {
	var documents []Document
	if err := s.db.Find(&documents, "knowledge_id = ?", knowledge.ID).Error; err != nil {
		return err
	}
	ragDocuments := make([]*rag.Document, len(documents))
	for i, doc := range documents {
		ragDocuments[i] = &rag.Document{
			Id:          doc.DocID,
			Title:       doc.Title,
			Content:     doc.Content,
			TextUnitIds: strings.Split(doc.TextUnitIDs, ","),
		}
	}
	wfCtx.Documents = append(wfCtx.Documents, ragDocuments...)
	return nil
}

func (s *Storage) loadTextUnits(_ context.Context, knowledge Knowledge, wfCtx *rag.WorkflowContext) error {
	var textUnits []TextUnit
	if err := s.db.Find(&textUnits, "knowledge_id = ?", knowledge.ID).Error; err != nil {
		return err
	}
	ragTextUnits := make([]*rag.TextUnit, len(textUnits))
	for i, unit := range textUnits {
		ragTextUnits[i] = &rag.TextUnit{
			Id:              unit.UnitID,
			Text:            unit.Text,
			DocumentIds:     strings.Split(unit.DocumentIDs, ","),
			EntityIds:       strings.Split(unit.EntityIDs, ","),
			RelationshipIds: strings.Split(unit.RelationshipIDs, ","),
			NumToken:        unit.NumToken,
		}
	}
	wfCtx.TextUnits = append(wfCtx.TextUnits, ragTextUnits...)
	return nil
}

func (s *Storage) loadEntities(_ context.Context, knowledge Knowledge, wfCtx *rag.WorkflowContext) error {
	var entities []Entity
	if err := s.db.Find(&entities, "knowledge_id = ?", knowledge.ID).Error; err != nil {
		return err
	}
	ragEntities := make([]*rag.Entity, len(entities))
	for i, entity := range entities {
		communities := make([]int, 0)
		for _, id := range strings.Split(entity.Communities, ",") {
			if id == "" {
				continue
			}
			comm, _ := strconv.Atoi(id)
			communities = append(communities, comm)
		}
		ragEntities[i] = &rag.Entity{
			Id:          entity.EntityID,
			Title:       entity.Title,
			Type:        entity.Type,
			Desc:        entity.Description,
			Degree:      entity.Degree,
			Communities: communities,
			TextUnitIds: strings.Split(entity.TextUnitIDs, ","),
		}
	}
	wfCtx.Entities = append(wfCtx.Entities, ragEntities...)
	return nil
}

func findEntityFromWfCtx(wfCtx *rag.WorkflowContext, entityId string) *rag.Entity {
	for _, entity := range wfCtx.Entities {
		if entity.Id == entityId {
			return entity
		}
	}
	return nil
}

func (s *Storage) loadRelationships(_ context.Context, knowledge Knowledge, wfCtx *rag.WorkflowContext) error {
	var relationships []Relationship
	if err := s.db.Find(&relationships, "knowledge_id = ?", knowledge.ID).Error; err != nil {
		return err
	}
	ragRelationships := make([]*rag.Relationship, len(relationships))
	for i, rel := range relationships {
		ragRelationships[i] = &rag.Relationship{
			Id:             rel.RelationshipID,
			Source:         findEntityFromWfCtx(wfCtx, rel.SourceEntityID),
			Target:         findEntityFromWfCtx(wfCtx, rel.TargetEntityID),
			Desc:           rel.Description,
			Weight:         rel.Weight,
			CombinedDegree: rel.CombinedDegree,
			TextUnitIds:    strings.Split(rel.TextUnitIDs, ","),
		}
	}
	wfCtx.Relationships = append(wfCtx.Relationships, ragRelationships...)
	return nil
}

func (s *Storage) loadTmpRelationships(_ context.Context, knowledge Knowledge, wfCtx *rag.WorkflowContext) error {
	var tmpRelationships []TmpRelationship
	if err := s.db.Find(&tmpRelationships, "knowledge_id = ?", knowledge.ID).Error; err != nil {
		return err
	}
	ragTmpRelationships := make([]*rag.TmpRelationship, len(tmpRelationships))
	for i, rel := range tmpRelationships {
		ragTmpRelationships[i] = &rag.TmpRelationship{
			Id:             rel.TmpRelationshipID,
			Source:         rel.Source,
			Target:         rel.Target,
			Desc:           rel.Description,
			Weight:         rel.Weight,
			CombinedDegree: rel.CombinedDegree,
			TextUnitIds:    strings.Split(rel.TextUnitIDs, ","),
			SourceId:       rel.SourceID,
		}
	}
	wfCtx.TmpRelationships = append(wfCtx.TmpRelationships, ragTmpRelationships...)
	return nil
}

func (s *Storage) loadNodes(_ context.Context, knowledge Knowledge, wfCtx *rag.WorkflowContext) error {
	var nodes []Node
	if err := s.db.Find(&nodes, "knowledge_id = ?", knowledge.ID).Error; err != nil {
		return err
	}
	ragNodes := make([]*rag.Node, len(nodes))
	for i, node := range nodes {
		ragNodes[i] = &rag.Node{
			Id:        node.NodeID,
			Title:     node.Title,
			Community: node.Community,
			Level:     node.Level,
			Degree:    node.Degree,
		}
	}
	wfCtx.Nodes = append(wfCtx.Nodes, ragNodes...)
	return nil
}

func (s *Storage) loadCommunities(_ context.Context, knowledge Knowledge, wfCtx *rag.WorkflowContext) error {
	var communities []Community
	if err := s.db.Find(&communities, "knowledge_id = ?", knowledge.ID).Error; err != nil {
		return err
	}
	ragCommunities := make([]*rag.Community, len(communities))
	for i, comm := range communities {
		ragCommunities[i] = &rag.Community{
			Id:              comm.CommunityID,
			Title:           comm.Title,
			Community:       comm.Community,
			Level:           comm.Level,
			RelationshipIds: strings.Split(comm.RelationshipIDs, ","),
			TextUnitIds:     strings.Split(comm.TextUnitIDs, ","),
			Parent:          comm.Parent,
			EntityIds:       strings.Split(comm.EntityIDs, ","),
			Period:          comm.Period,
			Size:            comm.Size,
		}
	}
	wfCtx.Communities = append(wfCtx.Communities, ragCommunities...)
	return nil
}

func (s *Storage) loadReports(_ context.Context, knowledge Knowledge, wfCtx *rag.WorkflowContext) error {
	var reports []Report
	if err := s.db.Find(&reports, "knowledge_id = ?", knowledge.ID).Error; err != nil {
		return err
	}
	reportIds := make([]int64, len(reports))
	for i, report := range reports {
		reportIds[i] = report.ID
	}
	var findings []Finding
	if err := s.db.Find(&findings, "report_id in ?", reportIds).Error; err != nil {
		return err
	}
	ragReports := make([]*rag.Report, len(reports))
	for i, report := range reports {
		ragFindings := make([]*rag.Finding, 0)
		for _, finding := range findings {
			if finding.ReportID == report.ID {
				ragFindings = append(ragFindings, &rag.Finding{
					Summary:     finding.Summary,
					Explanation: finding.Explanation,
				})
			}
		}
		ragReports[i] = &rag.Report{
			Community:         report.Community,
			Title:             report.Title,
			Summary:           report.Summary,
			Rating:            report.Rating,
			RatingExplanation: report.RatingExplanation,
			Findings:          ragFindings,
		}
	}
	wfCtx.Reports = append(wfCtx.Reports, ragReports...)
	return nil
}

func (s *Storage) deleteAllData(_ context.Context, knowledge Knowledge, tx *gorm.DB) error {
	err := tx.Delete(&Document{}, "knowledge_id = ?", knowledge.ID).Error
	if err != nil {
		return err
	}

	err = tx.Delete(&TextUnit{}, "knowledge_id = ?", knowledge.ID).Error
	if err != nil {
		return err
	}

	err = tx.Delete(&Relationship{}, "knowledge_id = ?", knowledge.ID).Error
	if err != nil {
		return err
	}

	err = tx.Delete(&TmpRelationship{}, "knowledge_id = ?", knowledge.ID).Error
	if err != nil {
		return err
	}

	err = tx.Delete(&Entity{}, "knowledge_id = ?", knowledge.ID).Error
	if err != nil {
		return err
	}

	err = tx.Delete(&Community{}, "knowledge_id = ?", knowledge.ID).Error
	if err != nil {
		return err
	}

	err = tx.Delete(&Node{}, "knowledge_id = ?", knowledge.ID).Error
	if err != nil {
		return err
	}

	var reports []Report
	if err := s.db.Find(&reports, "knowledge_id = ?", knowledge.ID).Error; err != nil {
		return err
	}
	reportIds := make([]int64, len(reports))
	for i, report := range reports {
		reportIds[i] = report.ID
	}

	err = tx.Delete(&Finding{}, "report_id in ?", reportIds).Error
	if err != nil {
		return err
	}

	err = tx.Delete(&Report{}, "knowledge_id = ?", knowledge.ID).Error
	if err != nil {
		return err
	}

	return nil
}

func batchCreate(tx *gorm.DB, data interface{}, batchSize int) error {
	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Slice {
		return fmt.Errorf("data must be a slice or a pointer to a slice")
	}
	length := v.Len()

	for i := 0; i < length; i += batchSize {
		end := i + batchSize
		if end > length {
			end = length
		}
		batch := v.Slice(i, end).Interface()
		if err := tx.Create(batch).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Storage) saveDocument(_ context.Context, wfCtx *rag.WorkflowContext, knowledge *Knowledge, tx *gorm.DB) error {
	if len(wfCtx.Documents) == 0 {
		return nil
	}
	documents := make([]Document, len(wfCtx.Documents))
	for i, ragDocument := range wfCtx.Documents {
		documents[i] = Document{
			DocID:       ragDocument.Id,
			Title:       ragDocument.Title,
			Content:     ragDocument.Content,
			TextUnitIDs: strings.Join(ragDocument.TextUnitIds, ","),
			KnowledgeId: knowledge.ID,
		}
	}
	if err := batchCreate(tx, &documents, 1000); err != nil {
		return err
	}
	ids := make([]int64, len(documents))
	for i, document := range documents {
		ids[i] = document.ID
	}
	return nil
}

func (s *Storage) saveTextUnit(_ context.Context, wfCtx *rag.WorkflowContext, knowledge *Knowledge, tx *gorm.DB) error {
	if len(wfCtx.TextUnits) == 0 {
		return nil
	}
	textUnits := make([]TextUnit, len(wfCtx.TextUnits))
	for i, ragTextUnit := range wfCtx.TextUnits {
		textUnits[i] = TextUnit{
			UnitID:          ragTextUnit.Id,
			Text:            ragTextUnit.Text,
			DocumentIDs:     strings.Join(ragTextUnit.DocumentIds, ","),
			EntityIDs:       strings.Join(ragTextUnit.EntityIds, ","),
			RelationshipIDs: strings.Join(ragTextUnit.RelationshipIds, ","),
			NumToken:        ragTextUnit.NumToken,
			KnowledgeId:     knowledge.ID,
		}
	}
	if err := batchCreate(tx, textUnits, 1000); err != nil {
		return err
	}
	ids := make([]int64, len(textUnits))
	for i, textUnit := range textUnits {
		ids[i] = textUnit.ID
	}
	return nil
}

func intsToStrings(ints []int) []string {
	strs := make([]string, len(ints))
	for i, v := range ints {
		strs[i] = strconv.Itoa(v)
	}
	return strs
}

func (s *Storage) saveEntity(_ context.Context, wfCtx *rag.WorkflowContext, knowledge *Knowledge, tx *gorm.DB) error {
	if len(wfCtx.Entities) == 0 {
		return nil
	}
	entities := make([]Entity, len(wfCtx.Entities))
	for i, ragEntity := range wfCtx.Entities {
		entities[i] = Entity{
			EntityID:    ragEntity.Id,
			Title:       ragEntity.Title,
			Type:        ragEntity.Type,
			Description: ragEntity.Desc,
			Degree:      ragEntity.Degree,
			Communities: strings.Join(intsToStrings(ragEntity.Communities), ","),
			TextUnitIDs: strings.Join(ragEntity.TextUnitIds, ","),
			KnowledgeId: knowledge.ID,
		}
	}
	if err := batchCreate(tx, entities, 1000); err != nil {
		return err
	}
	ids := make([]int64, len(entities))
	for i, entity := range entities {
		ids[i] = entity.ID
	}
	return nil
}

func (s *Storage) saveRelationship(_ context.Context, wfCtx *rag.WorkflowContext, knowledge *Knowledge, tx *gorm.DB) error {
	if len(wfCtx.Relationships) == 0 {
		return nil
	}
	relationships := make([]Relationship, len(wfCtx.Relationships))
	for i, ragRelationship := range wfCtx.Relationships {
		relationships[i] = Relationship{
			RelationshipID: ragRelationship.Id,
			SourceEntityID: ragRelationship.Source.Id,
			TargetEntityID: ragRelationship.Target.Id,
			Description:    ragRelationship.Desc,
			Weight:         ragRelationship.Weight,
			CombinedDegree: ragRelationship.CombinedDegree,
			TextUnitIDs:    strings.Join(ragRelationship.TextUnitIds, ","),
			KnowledgeId:    knowledge.ID,
		}
	}
	if err := batchCreate(tx, relationships, 1000); err != nil {
		return err
	}
	ids := make([]int64, len(relationships))
	for i, relationship := range relationships {
		ids[i] = relationship.ID
	}
	return nil
}

func (s *Storage) saveTmpRelationship(_ context.Context, wfCtx *rag.WorkflowContext, knowledge *Knowledge, tx *gorm.DB) error {
	if len(wfCtx.TmpRelationships) == 0 {
		return nil
	}
	tmpRelationships := make([]TmpRelationship, len(wfCtx.TmpRelationships))
	for i, ragTmpRelationship := range wfCtx.TmpRelationships {
		tmpRelationships[i] = TmpRelationship{
			TmpRelationshipID: ragTmpRelationship.Id,
			Source:            ragTmpRelationship.Source,
			Target:            ragTmpRelationship.Target,
			Description:       ragTmpRelationship.Desc,
			Weight:            ragTmpRelationship.Weight,
			CombinedDegree:    ragTmpRelationship.CombinedDegree,
			TextUnitIDs:       strings.Join(ragTmpRelationship.TextUnitIds, ","),
			SourceID:          ragTmpRelationship.SourceId,
			KnowledgeId:       knowledge.ID,
		}
	}
	if err := batchCreate(tx, tmpRelationships, 1000); err != nil {
		return err
	}
	ids := make([]int64, len(tmpRelationships))
	for i, tmpRelationship := range tmpRelationships {
		ids[i] = tmpRelationship.ID
	}
	return nil
}

func (s *Storage) saveNode(_ context.Context, wfCtx *rag.WorkflowContext, knowledge *Knowledge, tx *gorm.DB) error {
	if len(wfCtx.Nodes) == 0 {
		return nil
	}
	nodes := make([]Node, len(wfCtx.Nodes))
	for i, ragNode := range wfCtx.Nodes {
		nodes[i] = Node{
			NodeID:      ragNode.Id,
			Title:       ragNode.Title,
			Community:   ragNode.Community,
			Level:       ragNode.Level,
			Degree:      ragNode.Degree,
			KnowledgeId: knowledge.ID,
		}
	}
	if err := batchCreate(tx, nodes, 1000); err != nil {
		return err
	}
	ids := make([]int64, len(nodes))
	for i, node := range nodes {
		ids[i] = node.ID
	}
	return nil
}

func (s *Storage) saveCommunity(_ context.Context, wfCtx *rag.WorkflowContext, knowledge *Knowledge, tx *gorm.DB) error {
	if len(wfCtx.Communities) == 0 {
		return nil
	}
	communities := make([]Community, len(wfCtx.Communities))
	for i, ragCommunity := range wfCtx.Communities {
		communities[i] = Community{
			CommunityID:     ragCommunity.Id,
			Title:           ragCommunity.Title,
			Community:       ragCommunity.Community,
			Level:           ragCommunity.Level,
			RelationshipIDs: strings.Join(ragCommunity.RelationshipIds, ","),
			TextUnitIDs:     strings.Join(ragCommunity.TextUnitIds, ","),
			Parent:          ragCommunity.Parent,
			EntityIDs:       strings.Join(ragCommunity.EntityIds, ","),
			Period:          ragCommunity.Period,
			Size:            ragCommunity.Size,
			KnowledgeId:     knowledge.ID,
		}
	}
	if err := batchCreate(tx, communities, 500); err != nil {
		return err
	}
	ids := make([]int64, len(communities))
	for i, community := range communities {
		ids[i] = community.ID
	}
	return nil
}

func (s *Storage) saveReport(_ context.Context, wfCtx *rag.WorkflowContext, knowledge *Knowledge, tx *gorm.DB) error {
	if len(wfCtx.Reports) == 0 {
		return nil
	}
	reports := make([]Report, len(wfCtx.Reports))
	findings := make([]Finding, 0)
	for i, ragReport := range wfCtx.Reports {
		reports[i] = Report{
			Community:         ragReport.Community,
			Title:             ragReport.Title,
			Summary:           ragReport.Summary,
			Rating:            ragReport.Rating,
			RatingExplanation: ragReport.RatingExplanation,
			KnowledgeId:       knowledge.ID,
		}
	}
	if err := batchCreate(tx, reports, 1000); err != nil {
		return err
	}
	for i, report := range reports {
		for _, ragFinding := range wfCtx.Reports[i].Findings {
			findings = append(findings, Finding{
				ReportID:    report.ID,
				Summary:     ragFinding.Summary,
				Explanation: ragFinding.Explanation,
				KnowledgeId: knowledge.ID,
			})
		}
	}
	if err := batchCreate(tx, findings, 1000); err != nil {
		return err
	}
	ids := make([]int64, len(reports))
	for i, report := range reports {
		ids[i] = report.ID
	}
	return nil
}
