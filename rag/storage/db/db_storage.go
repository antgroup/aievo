package db

import (
	"context"
	"errors"
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

func (s *Storage) Load(ctx context.Context, wfCtx *rag.WorkflowContext) error {
	if wfCtx.Id == 0 {
		return errors.New("id is not set")
	}

	if wfCtx.Documents == nil || wfCtx.TextUnits == nil || wfCtx.Entities == nil ||
		wfCtx.Relationships == nil || wfCtx.TmpRelationships == nil || wfCtx.Nodes == nil ||
		wfCtx.Communities == nil || wfCtx.Reports == nil {
		return errors.New("please call rag.NewWorkflowContext() to create a new workflow context")
	}

	var knowledge Knowledge
	if err := s.db.Where("id = ?", wfCtx.Id).First(&knowledge).Error; err != nil {
		return err
	}

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

	return nil
}

func (s *Storage) Save(ctx context.Context, wfCtx *rag.WorkflowContext) error {
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

	documentIds, err := s.saveDocument(ctx, wfCtx, tx)
	if err != nil {
		tx.Rollback()
		return err
	}

	textUnitIds, err := s.saveTextUnit(ctx, wfCtx, tx)
	if err != nil {
		tx.Rollback()
		return err
	}

	entityIds, err := s.saveEntity(ctx, wfCtx, tx)
	if err != nil {
		tx.Rollback()
		return err
	}

	relationshipIds, err := s.saveRelationship(ctx, wfCtx, tx)
	if err != nil {
		tx.Rollback()
		return err
	}

	tmpRelationshipIds, err := s.saveTmpRelationship(ctx, wfCtx, tx)
	if err != nil {
		tx.Rollback()
		return err
	}

	nodeIds, err := s.saveNode(ctx, wfCtx, tx)
	if err != nil {
		tx.Rollback()
		return err
	}

	communityIds, err := s.saveCommunity(ctx, wfCtx, tx)
	if err != nil {
		tx.Rollback()
		return err
	}

	reportIds, err := s.saveReport(ctx, wfCtx, tx)
	if err != nil {
		tx.Rollback()
		return err
	}

	knowledge.DocumentIDs = int64ArrayToStr(documentIds)
	knowledge.TextUnitIDs = int64ArrayToStr(textUnitIds)
	knowledge.EntityIDs = int64ArrayToStr(entityIds)
	knowledge.RelationshipIDs = int64ArrayToStr(relationshipIds)
	knowledge.TmpRelationshipIDs = int64ArrayToStr(tmpRelationshipIds)
	knowledge.NodeIDs = int64ArrayToStr(nodeIds)
	knowledge.CommunityIDs = int64ArrayToStr(communityIds)
	knowledge.ReportIDs = int64ArrayToStr(reportIds)

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

func (s *Storage) loadDocuments(ctx context.Context, knowledge Knowledge, wfCtx *rag.WorkflowContext) error {
	var documents []Document
	documentIds := strings.Split(knowledge.DocumentIDs, ",")
	if err := s.db.Find(&documents, "id in ?", documentIds).Error; err != nil {
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

func (s *Storage) loadTextUnits(ctx context.Context, knowledge Knowledge, wfCtx *rag.WorkflowContext) error {
	var textUnits []TextUnit
	textUnitIds := strings.Split(knowledge.TextUnitIDs, ",")
	if err := s.db.Find(&textUnits, "id in ?", textUnitIds).Error; err != nil {
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

func (s *Storage) loadEntities(ctx context.Context, knowledge Knowledge, wfCtx *rag.WorkflowContext) error {
	var entities []Entity
	entityIds := strings.Split(knowledge.EntityIDs, ",")
	if err := s.db.Find(&entities, "id in ?", entityIds).Error; err != nil {
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

func (s *Storage) loadRelationships(ctx context.Context, knowledge Knowledge, wfCtx *rag.WorkflowContext) error {
	var relationships []Relationship
	relationshipIds := strings.Split(knowledge.RelationshipIDs, ",")
	if err := s.db.Find(&relationships, "id in ?", relationshipIds).Error; err != nil {
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

func (s *Storage) loadTmpRelationships(ctx context.Context, knowledge Knowledge, wfCtx *rag.WorkflowContext) error {
	var tmpRelationships []TmpRelationship
	tmpRelationshipIds := strings.Split(knowledge.TmpRelationshipIDs, ",")
	if err := s.db.Find(&tmpRelationships, "id in ?", tmpRelationshipIds).Error; err != nil {
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

func (s *Storage) loadNodes(ctx context.Context, knowledge Knowledge, wfCtx *rag.WorkflowContext) error {
	var nodes []Node
	nodeIds := strings.Split(knowledge.NodeIDs, ",")
	if err := s.db.Find(&nodes, "id in ?", nodeIds).Error; err != nil {
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

func (s *Storage) loadCommunities(ctx context.Context, knowledge Knowledge, wfCtx *rag.WorkflowContext) error {
	var communities []Community
	communityIds := strings.Split(knowledge.CommunityIDs, ",")
	if err := s.db.Find(&communities, "id in ?", communityIds).Error; err != nil {
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

func (s *Storage) loadReports(ctx context.Context, knowledge Knowledge, wfCtx *rag.WorkflowContext) error {
	var reports []Report
	reportIds := strings.Split(knowledge.ReportIDs, ",")
	if err := s.db.Find(&reports, "id in ?", reportIds).Error; err != nil {
		return err
	}
	ragReports := make([]*rag.Report, len(reports))
	for i, report := range reports {
		var findings []Finding
		if err := s.db.Where("report_id = ?", report.ID).Find(&findings).Error; err != nil {
			return err
		}
		ragFindings := make([]*rag.Finding, len(findings))
		for j, finding := range findings {
			ragFindings[j] = &rag.Finding{
				Summary:     finding.Summary,
				Explanation: finding.Explanation,
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
	documentIds := strings.Split(knowledge.DocumentIDs, ",")
	err := tx.Delete(&Document{}, "id in ?", documentIds).Error
	if err != nil {
		return err
	}

	textUnitIDs := strings.Split(knowledge.TextUnitIDs, ",")
	err = tx.Delete(&TextUnit{}, "id in ?", textUnitIDs).Error
	if err != nil {
		return err
	}

	relationshipIDs := strings.Split(knowledge.RelationshipIDs, ",")
	err = tx.Delete(&Relationship{}, "id in ?", relationshipIDs).Error
	if err != nil {
		return err
	}

	tmpRelationshipIDs := strings.Split(knowledge.TmpRelationshipIDs, ",")
	err = tx.Delete(&TmpRelationship{}, "id in ?", tmpRelationshipIDs).Error
	if err != nil {
		return err
	}

	entityIDs := strings.Split(knowledge.EntityIDs, ",")
	err = tx.Delete(&Entity{}, "id in ?", entityIDs).Error
	if err != nil {
		return err
	}

	communityIDs := strings.Split(knowledge.CommunityIDs, ",")
	err = tx.Delete(&Community{}, "id in ?", communityIDs).Error
	if err != nil {
		return err
	}

	nodeIDs := strings.Split(knowledge.NodeIDs, ",")
	err = tx.Delete(&Node{}, "id in ?", nodeIDs).Error
	if err != nil {
		return err
	}

	reportIDs := strings.Split(knowledge.ReportIDs, ",")
	err = tx.Delete(&Finding{}, "report_id in ?", reportIDs).Error
	if err != nil {
		return err
	}
	err = tx.Delete(&Report{}, "id in ?", reportIDs).Error
	if err != nil {
		return err
	}

	return nil
}

func (s *Storage) saveDocument(_ context.Context, wfCtx *rag.WorkflowContext, tx *gorm.DB) ([]int64, error) {
	if len(wfCtx.Documents) == 0 {
		return nil, nil
	}
	documents := make([]Document, len(wfCtx.Documents))
	for i, ragDocument := range wfCtx.Documents {
		documents[i] = Document{
			DocID:       ragDocument.Id,
			Title:       ragDocument.Title,
			Content:     ragDocument.Content,
			TextUnitIDs: strings.Join(ragDocument.TextUnitIds, ","),
		}
	}
	if err := tx.Create(&documents).Error; err != nil {
		return nil, err
	}
	ids := make([]int64, len(documents))
	for i, document := range documents {
		ids[i] = document.ID
	}
	return ids, nil
}

func (s *Storage) saveTextUnit(_ context.Context, wfCtx *rag.WorkflowContext, tx *gorm.DB) ([]int64, error) {
	if len(wfCtx.TextUnits) == 0 {
		return nil, nil
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
		}
	}
	if err := tx.Create(&textUnits).Error; err != nil {
		return nil, err
	}
	ids := make([]int64, len(textUnits))
	for i, textUnit := range textUnits {
		ids[i] = textUnit.ID
	}
	return ids, nil
}

func intsToStrings(ints []int) []string {
	strs := make([]string, len(ints))
	for i, v := range ints {
		strs[i] = strconv.Itoa(v)
	}
	return strs
}

func (s *Storage) saveEntity(_ context.Context, wfCtx *rag.WorkflowContext, tx *gorm.DB) ([]int64, error) {
	if len(wfCtx.Entities) == 0 {
		return nil, nil
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
		}
	}
	if err := tx.Create(&entities).Error; err != nil {
		return nil, err
	}
	ids := make([]int64, len(entities))
	for i, entity := range entities {
		ids[i] = entity.ID
	}
	return ids, nil
}

func (s *Storage) saveRelationship(_ context.Context, wfCtx *rag.WorkflowContext, tx *gorm.DB) ([]int64, error) {
	if len(wfCtx.Relationships) == 0 {
		return nil, nil
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
		}
	}
	if err := tx.Create(&relationships).Error; err != nil {
		return nil, err
	}
	ids := make([]int64, len(relationships))
	for i, relationship := range relationships {
		ids[i] = relationship.ID
	}
	return ids, nil
}

func (s *Storage) saveTmpRelationship(_ context.Context, wfCtx *rag.WorkflowContext, tx *gorm.DB) ([]int64, error) {
	if len(wfCtx.TmpRelationships) == 0 {
		return nil, nil
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
		}
	}
	if err := tx.Create(&tmpRelationships).Error; err != nil {
		return nil, err
	}
	ids := make([]int64, len(tmpRelationships))
	for i, tmpRelationship := range tmpRelationships {
		ids[i] = tmpRelationship.ID
	}
	return ids, nil
}

func (s *Storage) saveNode(_ context.Context, wfCtx *rag.WorkflowContext, tx *gorm.DB) ([]int64, error) {
	if len(wfCtx.Nodes) == 0 {
		return nil, nil
	}
	nodes := make([]Node, len(wfCtx.Nodes))
	for i, ragNode := range wfCtx.Nodes {
		nodes[i] = Node{
			NodeID:    ragNode.Id,
			Title:     ragNode.Title,
			Community: ragNode.Community,
			Level:     ragNode.Level,
			Degree:    ragNode.Degree,
		}
	}
	if err := tx.Create(&nodes).Error; err != nil {
		return nil, err
	}
	ids := make([]int64, len(nodes))
	for i, node := range nodes {
		ids[i] = node.ID
	}
	return ids, nil
}

func (s *Storage) saveCommunity(_ context.Context, wfCtx *rag.WorkflowContext, tx *gorm.DB) ([]int64, error) {
	if len(wfCtx.Communities) == 0 {
		return nil, nil
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
		}
	}
	if err := tx.Create(&communities).Error; err != nil {
		return nil, err
	}
	ids := make([]int64, len(communities))
	for i, community := range communities {
		ids[i] = community.ID
	}
	return ids, nil
}

func (s *Storage) saveReport(_ context.Context, wfCtx *rag.WorkflowContext, tx *gorm.DB) ([]int64, error) {
	if len(wfCtx.Reports) == 0 {
		return nil, nil
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
		}
	}
	if err := tx.Create(&reports).Error; err != nil {
		return nil, err
	}
	for i, report := range reports {
		for _, ragFinding := range wfCtx.Reports[i].Findings {
			findings = append(findings, Finding{
				ReportID:    report.ID,
				Summary:     ragFinding.Summary,
				Explanation: ragFinding.Explanation,
			})
		}
	}
	if err := tx.Create(&findings).Error; err != nil {
		return nil, err
	}
	ids := make([]int64, len(reports))
	for i, report := range reports {
		ids[i] = report.ID
	}
	return ids, nil
}
