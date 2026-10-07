CREATE TABLE IF NOT EXISTS grades (
    id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    code        VARCHAR(10)     NOT NULL,
    scheme      VARCHAR(10)     NOT NULL,
    level       INT             NOT NULL,
    title       VARCHAR(150)    NOT NULL,
    created_at  DATETIME(6)     NOT NULL,
    updated_at  DATETIME(6)     NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_grades_code (code),
    KEY idx_grades_scheme_level (scheme, level)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS employees (
    id                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    employee_no       VARCHAR(30)     NOT NULL,
    ic_number         CHAR(12)        NOT NULL,
    full_name         VARCHAR(200)    NOT NULL,
    email             VARCHAR(255)    NOT NULL,
    phone             VARCHAR(30)     NOT NULL DEFAULT '',
    date_of_birth     DATE            NOT NULL,
    gender            VARCHAR(10)     NOT NULL,
    current_grade_id  BIGINT UNSIGNED NOT NULL,
    position          VARCHAR(150)    NOT NULL,
    department        VARCHAR(150)    NOT NULL,
    service_status    VARCHAR(20)     NOT NULL DEFAULT 'ACTIVE',
    date_joined       DATE            NOT NULL,
    confirmed_at      DATE            NULL,
    created_at        DATETIME(6)     NOT NULL,
    updated_at        DATETIME(6)     NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_employees_employee_no (employee_no),
    UNIQUE KEY uq_employees_ic_number (ic_number),
    UNIQUE KEY uq_employees_email (email),
    KEY idx_employees_department (department),
    KEY idx_employees_status (service_status),
    CONSTRAINT fk_employees_grade FOREIGN KEY (current_grade_id) REFERENCES grades (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS users (
    id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    username       VARCHAR(50)     NOT NULL,
    password_hash  VARCHAR(255)    NOT NULL,
    role           VARCHAR(20)     NOT NULL,
    employee_id    BIGINT UNSIGNED NULL,
    is_active      TINYINT(1)      NOT NULL DEFAULT 1,
    created_at     DATETIME(6)     NOT NULL,
    updated_at     DATETIME(6)     NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_users_username (username),
    UNIQUE KEY uq_users_employee (employee_id),
    CONSTRAINT fk_users_employee FOREIGN KEY (employee_id) REFERENCES employees (id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS service_records (
    id                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    employee_id       BIGINT UNSIGNED NOT NULL,
    grade_id          BIGINT UNSIGNED NOT NULL,
    position          VARCHAR(150)    NOT NULL,
    department        VARCHAR(150)    NOT NULL,
    appointment_type  VARCHAR(20)     NOT NULL,
    effective_from    DATE            NOT NULL,
    effective_to      DATE            NULL,
    remarks           VARCHAR(1000)   NOT NULL DEFAULT '',
    -- 1 for the current record, NULL otherwise; the unique key below allows
    -- many NULLs but only one current record per employee.
    current_flag      TINYINT GENERATED ALWAYS AS (IF(effective_to IS NULL, 1, NULL)) STORED,
    created_at        DATETIME(6)     NOT NULL,
    updated_at        DATETIME(6)     NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_service_records_current (employee_id, current_flag),
    KEY idx_service_records_employee_from (employee_id, effective_from),
    CONSTRAINT fk_service_records_employee FOREIGN KEY (employee_id) REFERENCES employees (id),
    CONSTRAINT fk_service_records_grade FOREIGN KEY (grade_id) REFERENCES grades (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS promotion_applications (
    id                 BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    employee_id        BIGINT UNSIGNED NOT NULL,
    current_grade_id   BIGINT UNSIGNED NOT NULL,
    proposed_grade_id  BIGINT UNSIGNED NOT NULL,
    justification      VARCHAR(2000)   NOT NULL,
    status             VARCHAR(10)     NOT NULL DEFAULT 'PENDING',
    submitted_at       DATETIME(6)     NOT NULL,
    reviewed_by        BIGINT UNSIGNED NULL,
    reviewed_at        DATETIME(6)     NULL,
    review_remarks     VARCHAR(1000)   NOT NULL DEFAULT '',
    effective_date     DATE            NULL,
    -- Same trick as service_records: at most one PENDING application per employee.
    pending_flag       TINYINT GENERATED ALWAYS AS (IF(status = 'PENDING', 1, NULL)) STORED,
    created_at         DATETIME(6)     NOT NULL,
    updated_at         DATETIME(6)     NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_promotion_applications_pending (employee_id, pending_flag),
    KEY idx_promotion_applications_status (status, submitted_at),
    CONSTRAINT fk_promotion_applications_employee FOREIGN KEY (employee_id) REFERENCES employees (id),
    CONSTRAINT fk_promotion_applications_current_grade FOREIGN KEY (current_grade_id) REFERENCES grades (id),
    CONSTRAINT fk_promotion_applications_proposed_grade FOREIGN KEY (proposed_grade_id) REFERENCES grades (id),
    CONSTRAINT fk_promotion_applications_reviewer FOREIGN KEY (reviewed_by) REFERENCES users (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS disciplinary_records (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    employee_id     BIGINT UNSIGNED NOT NULL,
    case_no         VARCHAR(50)     NOT NULL,
    category        VARCHAR(10)     NOT NULL,
    description     VARCHAR(4000)   NOT NULL,
    incident_date   DATE            NOT NULL,
    penalty         VARCHAR(30)     NOT NULL,
    status          VARCHAR(10)     NOT NULL DEFAULT 'PENDING',
    reported_by     BIGINT UNSIGNED NOT NULL,
    reviewed_by     BIGINT UNSIGNED NULL,
    reviewed_at     DATETIME(6)     NULL,
    review_remarks  VARCHAR(1000)   NOT NULL DEFAULT '',
    created_at      DATETIME(6)     NOT NULL,
    updated_at      DATETIME(6)     NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_disciplinary_records_case_no (case_no),
    KEY idx_disciplinary_records_employee_status (employee_id, status, reviewed_at),
    CONSTRAINT fk_disciplinary_records_employee FOREIGN KEY (employee_id) REFERENCES employees (id),
    CONSTRAINT fk_disciplinary_records_reporter FOREIGN KEY (reported_by) REFERENCES users (id),
    CONSTRAINT fk_disciplinary_records_reviewer FOREIGN KEY (reviewed_by) REFERENCES users (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
