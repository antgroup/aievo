-- 文档表
CREATE TABLE aievo_rag_document
(
    id            BIGINT PRIMARY KEY AUTO_INCREMENT,
    doc_id        VARCHAR(64)  NOT NULL COMMENT '原始文档ID',
    title         VARCHAR(255) NOT NULL COMMENT '文档标题',
    content       TEXT         NOT NULL COMMENT '文档内容',
    text_unit_ids TEXT COMMENT '逗号分隔的text_unit_ids',
    knowledge_id  INT          NOT NULL DEFAULT 0 COMMENT '知识库ID',
    gmt_create    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    gmt_modified  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
    INDEX         idx_doc_id (doc_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='文档表';

-- 文本单元表
CREATE TABLE aievo_rag_textunit
(
    id               BIGINT PRIMARY KEY AUTO_INCREMENT,
    unit_id          VARCHAR(64) NOT NULL COMMENT '原始文本单元ID',
    text             TEXT        NOT NULL COMMENT '文本内容',
    document_ids     TEXT COMMENT '逗号分隔的document_ids',
    entity_ids       TEXT COMMENT '逗号分隔的entity_ids',
    relationship_ids TEXT COMMENT '逗号分隔的relationship_ids',
    num_token        INT         NOT NULL DEFAULT 0 COMMENT '文本token数量',
    knowledge_id     INT         NOT NULL DEFAULT 0 COMMENT '知识库ID',
    gmt_create       TIMESTAMP   NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    gmt_modified     TIMESTAMP   NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
    INDEX            idx_unit_id (unit_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='文本单元表';

-- 实体表
CREATE TABLE aievo_rag_entity
(
    id            BIGINT PRIMARY KEY AUTO_INCREMENT,
    entity_id     VARCHAR(64)  NOT NULL COMMENT '原始实体ID',
    title         VARCHAR(255) NOT NULL COMMENT '实体标题',
    type          VARCHAR(64)  NOT NULL COMMENT '实体类型',
    description   TEXT COMMENT '实体描述',
    degree        INT          NOT NULL DEFAULT 0 COMMENT '度数',
    communities   TEXT COMMENT '逗号分隔的community ids',
    text_unit_ids TEXT COMMENT '逗号分隔的text_unit_ids',
    knowledge_id  INT          NOT NULL DEFAULT 0 COMMENT '知识库ID',
    gmt_create    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    gmt_modified  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
    INDEX         idx_entity_id (entity_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='实体表';

-- 关系表
CREATE TABLE aievo_rag_relationship
(
    id               BIGINT PRIMARY KEY AUTO_INCREMENT,
    relationship_id  VARCHAR(64)    NOT NULL COMMENT '原始关系ID',
    source_entity_id VARCHAR(64)    NOT NULL COMMENT '源实体ID',
    target_entity_id VARCHAR(64)    NOT NULL COMMENT '目标实体ID',
    description      TEXT COMMENT '关系描述',
    weight           DECIMAL(10, 4) NOT NULL DEFAULT 0 COMMENT '权重',
    combined_degree  INT            NOT NULL DEFAULT 0 COMMENT '组合度数',
    text_unit_ids    TEXT COMMENT '逗号分隔的text_unit_ids',
    knowledge_id     INT            NOT NULL DEFAULT 0 COMMENT '知识库ID',
    gmt_create       TIMESTAMP      NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    gmt_modified     TIMESTAMP      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
    INDEX            idx_relationship_id (relationship_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='关系表';

-- 临时关系表
CREATE TABLE aievo_rag_tmprelationship
(
    id                  BIGINT PRIMARY KEY AUTO_INCREMENT,
    tmp_relationship_id VARCHAR(64)    NOT NULL COMMENT '原始临时关系ID',
    source              VARCHAR(255)   NOT NULL COMMENT '源',
    target              VARCHAR(255)   NOT NULL COMMENT '目标',
    description         TEXT COMMENT '关系描述',
    weight              DECIMAL(10, 4) NOT NULL DEFAULT 0 COMMENT '权重',
    combined_degree     INT            NOT NULL DEFAULT 0 COMMENT '组合度数',
    text_unit_ids       TEXT COMMENT '逗号分隔的text_unit_ids',
    source_id           VARCHAR(64)    NOT NULL COMMENT '源ID',
    knowledge_id        INT            NOT NULL DEFAULT 0 COMMENT '知识库ID',
    gmt_create          TIMESTAMP      NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    gmt_modified        TIMESTAMP      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
    INDEX               idx_tmp_relationship_id (tmp_relationship_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='临时关系表';

-- 节点表
CREATE TABLE aievo_rag_node
(
    id           BIGINT PRIMARY KEY AUTO_INCREMENT,
    node_id      VARCHAR(64)  NOT NULL COMMENT '原始节点ID',
    title        VARCHAR(255) NOT NULL COMMENT '节点标题',
    community    INT          NOT NULL COMMENT '社区ID',
    level        INT          NOT NULL COMMENT '层级',
    degree       INT          NOT NULL DEFAULT 0 COMMENT '度数',
    knowledge_id INT          NOT NULL DEFAULT 0 COMMENT '知识库ID',
    gmt_create   TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    gmt_modified TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
    INDEX        idx_node_id (node_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='节点表';

-- 社区表
CREATE TABLE aievo_rag_community
(
    id               BIGINT PRIMARY KEY AUTO_INCREMENT,
    community_id     VARCHAR(64)  NOT NULL COMMENT '原始社区ID',
    title            VARCHAR(255) NOT NULL COMMENT '社区标题',
    community        INT          NOT NULL COMMENT '社区ID',
    level            INT          NOT NULL COMMENT '层级',
    relationship_ids TEXT COMMENT '逗号分隔的relationship_ids',
    text_unit_ids    TEXT COMMENT '逗号分隔的text_unit_ids',
    parent           INT          NOT NULL DEFAULT 0 COMMENT '父级ID',
    entity_ids       TEXT COMMENT '逗号分隔的entity_ids',
    period VARCHAR (64) COMMENT '时期',
    size             INT          NOT NULL DEFAULT 0 COMMENT '大小',
    knowledge_id     INT          NOT NULL DEFAULT 0 COMMENT '知识库ID',
    gmt_create       TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    gmt_modified     TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
    INDEX            idx_community_id (community_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='社区表';

-- 报告表
CREATE TABLE aievo_rag_report
(
    id                 BIGINT PRIMARY KEY AUTO_INCREMENT,
    community          INT            NOT NULL COMMENT '社区ID',
    title              VARCHAR(255)   NOT NULL COMMENT '报告标题',
    summary            TEXT COMMENT '报告摘要',
    rating             DECIMAL(10, 4) NOT NULL DEFAULT 0 COMMENT '评分',
    rating_explanation TEXT COMMENT '评分说明',
    knowledge_id       INT            NOT NULL DEFAULT 0 COMMENT '知识库ID',
    gmt_create         TIMESTAMP      NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    gmt_modified       TIMESTAMP      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='报告表';

-- 报告发现表
CREATE TABLE aievo_rag_finding
(
    id           BIGINT PRIMARY KEY AUTO_INCREMENT,
    report_id    BIGINT    NOT NULL COMMENT '关联的报告ID',
    summary      TEXT COMMENT '发现摘要',
    explanation  TEXT COMMENT '发现说明',
    knowledge_id INT       NOT NULL DEFAULT 0 COMMENT '知识库ID',
    gmt_create   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    gmt_modified TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='报告发现表';

-- 知识汇总表
CREATE TABLE aievo_rag_knowledge
(
    id             BIGINT PRIMARY KEY AUTO_INCREMENT,
    name           VARCHAR(255) NOT NULL COMMENT '知识库名称',
    description    TEXT COMMENT '知识库描述',
    type           VARCHAR(64)  NOT NULL COMMENT '知识库类型',
    status         VARCHAR(64)  NOT NULL COMMENT '知识库状态',
    creator        VARCHAR(100) NOT NULL COMMENT '创建人',
    knowledge_id   INT          NOT NULL DEFAULT 0 COMMENT '知识库ID',
    index_progress INT COMMENT  '索引进度',
    gmt_create     TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    gmt_modified   TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='知识汇总表';

-- 知识汇总表
CREATE TABLE aievo_rag_knowledge
(
    id             BIGINT PRIMARY KEY AUTO_INCREMENT,
    name           VARCHAR(255) NOT NULL COMMENT '知识库名称',
    index_progress INT COMMENT  '索引进度',
    gmt_create     TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    gmt_modified   TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='知识汇总表';