// Package skills auto-detects languages, frameworks, and domains from changed files
// and loads only the relevant skill modules into the system prompt.
package skills

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ── Mappings ────────────────────────────────────────────────────────────────

var languageMap = map[string]string{
	".py": "python", ".pyx": "python", ".pyi": "python",
	".java": "java", ".kt": "java", ".kts": "java", ".scala": "java", ".groovy": "java",
	".go": "golang",
	".ts": "typescript", ".tsx": "typescript", ".js": "typescript", ".jsx": "typescript",
	".mjs": "typescript", ".cjs": "typescript",
	".c": "cpp", ".cpp": "cpp", ".cc": "cpp", ".cxx": "cpp", ".h": "cpp",
	".hpp": "cpp", ".hxx": "cpp",
	".rs": "rust", ".cs": "csharp",
	".rb": "ruby", ".erb": "ruby",
	".php": "php", ".swift": "swift",
	".sql": "sql",
	".sh": "shell", ".bash": "shell", ".zsh": "shell",
	".cob": "legacy", ".cbl": "legacy", ".cpy": "legacy",
	".f": "legacy", ".f90": "legacy", ".f95": "legacy",
	".abap": "legacy",
}

type indicator struct {
	imports []string
	files   []string
}

var frameworkIndicators = map[string]indicator{
	"django":  {imports: []string{"django", "from django"}, files: []string{"manage.py", "settings.py", "wsgi.py", "asgi.py"}},
	"fastapi": {imports: []string{"fastapi", "from fastapi"}},
	"flask":   {imports: []string{"flask", "from flask"}},
	"react":   {imports: []string{"react", "from react", "'react'", "\"react\""}},
	"nextjs":  {imports: []string{"next/", "from next"}, files: []string{"next.config.js", "next.config.ts", "next.config.mjs"}},
	"express": {imports: []string{"express", "from express"}},
	"nestjs":  {imports: []string{"@nestjs/"}},
	"vue":     {imports: []string{"vue", "from vue"}},
	"angular": {imports: []string{"@angular/"}, files: []string{"angular.json"}},
	"spring":  {imports: []string{"org.springframework", "spring-boot"}, files: []string{"pom.xml", "build.gradle"}},
	"rails":   {imports: []string{"rails", "activerecord"}, files: []string{"Gemfile", "Rakefile"}},
	"dotnet":  {imports: []string{"Microsoft.AspNetCore", "System.Linq", "Microsoft.EntityFrameworkCore"}},
}

type devopsIndicator struct {
	files           []string
	extensions      []string
	contentPatterns []string
	dirPatterns     []string
}

var devopsIndicators = map[string]devopsIndicator{
	"docker":     {files: []string{"Dockerfile", "docker-compose.yml", "docker-compose.yaml", ".dockerignore"}},
	"kubernetes": {extensions: []string{".yaml", ".yml"}, contentPatterns: []string{"apiVersion:", "kind: Deployment", "kind: Service", "kind: Pod", "kind: ConfigMap", "kind: Ingress"}},
	"helm":       {files: []string{"Chart.yaml", "Chart.yml", "values.yaml", "values.yml"}, extensions: []string{".tpl"}, dirPatterns: []string{"charts/", "templates/"}},
	"terraform":  {extensions: []string{".tf", ".tfvars"}},
	"cicd":       {files: []string{".github/workflows/", ".gitlab-ci.yml", "Jenkinsfile", ".circleci/config.yml", ".travis.yml", "azure-pipelines.yml"}},
	"ansible":    {files: []string{"playbook.yml", "ansible.cfg", "inventory"}, dirPatterns: []string{"roles/", "playbooks/"}},
}

type domainDef struct {
	imports  []string
	files    []string
	skillFile string
}

var domainIndicators = map[string]domainDef{
	"fintech": {
		imports:  []string{"stripe", "razorpay", "paypal", "braintree", "adyen", "payment", "checkout", "billing", "invoice", "ledger", "accounting", "loan", "emi", "amortization", "disbursement", "insurance", "policy", "premium", "claim", "underwriting", "kyc", "aml", "pci", "wallet", "upi"},
		files:    []string{"payment", "checkout", "billing", "loan", "emi", "lending", "insurance", "policy", "claim", "ledger", "reconcil", "kyc", "aml", "wallet", "refund", "settlement"},
		skillFile: "fintech",
	},
	"india_regulatory": {
		imports:  []string{"upi", "npci", "nach", "ecs", "neft", "rtgs", "imps", "aadhaar", "uidai", "digilocker", "ckyc", "ekyc", "ifsc", "rbi", "nbfc", "cersai", "razorpay", "paytm", "phonepe", "cashfree", "account_aggregator"},
		files:    []string{"upi", "nach", "ecs", "neft", "rtgs", "imps", "aadhaar", "ekyc", "ckyc", "nbfc", "rbi", "mandate"},
		skillFile: "india-regulatory",
	},
	"credit_bureau": {
		imports:  []string{"cibil", "transunion", "experian", "equifax", "crif", "credit_score", "credit_report", "bureau", "credit_bureau"},
		files:    []string{"bureau", "cibil", "credit_score", "credit_report"},
		skillFile: "credit-bureau",
	},
	"fraud": {
		imports:  []string{"fraud", "risk_score", "risk_engine", "velocity_check", "device_fingerprint", "anti_fraud", "aml", "money_laundering", "chargeback", "dispute"},
		files:    []string{"fraud", "risk_engine", "risk_score", "anti_fraud", "velocity", "chargeback", "dispute"},
		skillFile: "fraud",
	},
	"notifications": {
		imports:  []string{"sms", "twilio", "msg91", "kaleyra", "gupshup", "sendgrid", "ses", "mailgun", "fcm", "apns", "push_notification", "whatsapp", "dlt", "trai"},
		files:    []string{"notification", "sms", "email_service", "push", "whatsapp", "communication"},
		skillFile: "notifications",
	},
	"financial_microservices": {
		imports:  []string{"saga", "compensat", "outbox", "event_sourcing", "distributed_transaction", "circuit_breaker", "kafka", "rabbitmq", "ledger_service", "payment_service"},
		files:    []string{"saga", "orchestrat", "compensat", "outbox", "event_store", "ledger_service", "settlement"},
		skillFile: "financial-microservices",
	},
	"data_privacy": {
		imports:  []string{"gdpr", "ccpa", "dpdp", "privacy", "consent", "data_subject", "erasure", "anonymiz", "pseudonymiz", "pii", "personal_data", "data_protection"},
		files:    []string{"privacy", "consent", "gdpr", "ccpa", "dpdp", "anonymiz", "pii", "data_protection", "data_deletion"},
		skillFile: "data-privacy",
	},
	"airflow": {
		imports:  []string{"airflow", "from airflow", "DAG", "PythonOperator", "BashOperator", "TaskGroup", "XCom", "BaseOperator", "dag_id", "schedule_interval"},
		files:    []string{"dag", "airflow", "dags/", "pipeline"},
		skillFile: "airflow",
	},
	"kafka": {
		imports:  []string{"kafka", "confluent_kafka", "kafka-python", "aiokafka", "KafkaProducer", "KafkaConsumer", "kafka.clients", "org.apache.kafka", "kafkajs", "node-rdkafka"},
		files:    []string{"kafka", "producer", "consumer", "topic", "broker"},
		skillFile: "kafka",
	},
	"messaging": {
		imports:  []string{"rabbitmq", "amqp", "pika", "celery", "kombu", "bullmq", "bull", "amqplib", "sqs", "SendMessage", "ReceiveMessage", "rq", "redis_queue", "huey"},
		files:    []string{"rabbitmq", "celery", "worker", "queue", "tasks.py", "celeryconfig", "sqs", "bullmq"},
		skillFile: "messaging",
	},
	"sql": {
		imports:  []string{"select ", "insert into", "update ", "delete from", " join ", "cursor.execute", "db.query", "conn.exec", "executemany"},
		files:    []string{".sql"},
		skillFile: "sql",
	},
	"migrations": {
		imports:  []string{"alembic", "from alembic", "flyway", "liquibase", "knex.migrate", "prisma migrate", "ActiveRecord::Migration"},
		files:    []string{"migrations/", "db/migrate/", "alembic/versions/", "flyway/", "liquibase/", "_migration.py", ".changeset.xml", "schema_migrations"},
		skillFile: "migrations",
	},
	"orm": {
		imports:  []string{"django.db", "sqlalchemy", "from sqlalchemy", "prisma", "@prisma/client", "typeorm", "sequelize", "mongoose", "gorm", "hibernate", "@entity", "javax.persistence", "jakarta.persistence", "spring-data"},
		files:    []string{"models.py", "entity.ts", "entity.js", ".repository.ts", ".repository.java"},
		skillFile: "orm",
	},
	"transactions": {
		imports:  []string{"transaction.atomic", "@transactional", "begintx", "select for update", "for no key update", "pg_advisory_lock", "advisory_xact_lock", "savepoint ", "db.transaction(", "session.starttransaction"},
		skillFile: "transactions",
	},
	"postgres": {
		imports:  []string{"psycopg2", "asyncpg", "node-postgres", "from pg ", "pgx", "gorm.io/driver/postgres", "jsonb", "on conflict", "concurrently", "tsvector", "pg_trgm", "pg_stat_statements"},
		files:    []string{"postgres", "postgresql"},
		skillFile: "postgres",
	},
	"redis": {
		imports:  []string{"redis", "ioredis", "go-redis", "lettuce", "jedis", "stackexchange.redis", "setnx", "xadd", "xreadgroup", "subscribe", "redis.lua", "redis.eval"},
		files:    []string{"redis"},
		skillFile: "redis",
	},
	"mongodb": {
		imports:  []string{"pymongo", "mongoose", "mongodb", "mongoclient", "@document", "$match", "$lookup", "$group", "$unwind", "findoneandupdate", "aggregate(", "bulkwrite"},
		files:    []string{"mongo"},
		skillFile: "mongodb",
	},
	"elasticsearch": {
		imports:  []string{"elasticsearch", "@elastic/elasticsearch", "opensearch", "org.elasticsearch.client", "_search", "_bulk", "_mapping", "indices.put_mapping", "search_after"},
		files:    []string{"elastic", "opensearch"},
		skillFile: "elasticsearch",
	},
}

var alwaysLoad = []string{"commandments", "security", "scalability", "stability", "maintainability", "performance"}

// ── Detection ───────────────────────────────────────────────────────────────

type Detected struct {
	Languages  []string
	Frameworks []string
	DevOps     []string
	Domains    []string
	Always     []string
}

func Detect(changedFiles []string, fileContents string) Detected {
	langs := map[string]bool{}
	fws := map[string]bool{}
	devops := map[string]bool{}
	domains := map[string]bool{}
	contentLower := strings.ToLower(fileContents)

	for _, fp := range changedFiles {
		ext := strings.ToLower(filepath.Ext(fp))
		name := strings.ToLower(filepath.Base(fp))
		fpLower := strings.ToLower(fp)

		if lang, ok := languageMap[ext]; ok {
			langs[lang] = true
		}
		for dtype, ind := range devopsIndicators {
			for _, p := range ind.files {
				if name == strings.ToLower(p) || strings.HasSuffix(fpLower, strings.ToLower(p)) {
					devops[dtype] = true
				}
			}
			for _, e := range ind.extensions {
				if ext == e {
					devops[dtype] = true
				}
			}
			for _, d := range ind.dirPatterns {
				if strings.Contains(fpLower, strings.ToLower(d)) {
					devops[dtype] = true
				}
			}
		}
		for fw, ind := range frameworkIndicators {
			for _, p := range ind.files {
				if name == strings.ToLower(p) || strings.HasSuffix(fpLower, strings.ToLower(p)) {
					fws[fw] = true
				}
			}
		}
	}

	if fileContents != "" {
		for fw, ind := range frameworkIndicators {
			for _, imp := range ind.imports {
				if strings.Contains(contentLower, strings.ToLower(imp)) {
					fws[fw] = true
					break
				}
			}
		}
		for _, di := range devopsIndicators {
			for _, p := range di.contentPatterns {
				if strings.Contains(contentLower, strings.ToLower(p)) {
					devops["kubernetes"] = true
					break
				}
			}
		}
		for dname, dd := range domainIndicators {
			found := false
			for _, imp := range dd.imports {
				if strings.Contains(contentLower, strings.ToLower(imp)) {
					found = true
					break
				}
			}
			if !found {
				for _, fp := range dd.files {
					for _, cf := range changedFiles {
						if strings.Contains(strings.ToLower(cf), strings.ToLower(fp)) {
							found = true
							break
						}
					}
					if found {
						break
					}
				}
			}
			if found {
				domains[dname] = true
			}
		}
	}

	return Detected{
		Languages:  sortedKeys(langs),
		Frameworks: sortedKeys(fws),
		DevOps:     sortedKeys(devops),
		Domains:    sortedKeys(domains),
		Always:     alwaysLoad,
	}
}

// ── Loading ─────────────────────────────────────────────────────────────────

func skillDirs() []string {
	var dirs []string
	if info, err := os.Stat(filepath.Join(".pr-review", "skills")); err == nil && info.IsDir() {
		dirs = append(dirs, filepath.Join(".pr-review", "skills"))
	}
	return dirs
}

func loadSkillFile(name string) string {
	for _, dir := range skillDirs() {
		data, err := os.ReadFile(filepath.Join(dir, name+".md"))
		if err == nil {
			return string(data)
		}
	}
	return ""
}

func LoadSkills(d Detected) string {
	var sections []string

	for _, name := range d.Always {
		if c := loadSkillFile(name); c != "" {
			sections = append(sections, c)
		}
	}
	if len(d.Languages) > 0 {
		if c := loadSkillFile("languages"); c != "" {
			if rel := extractSections(c, d.Languages); rel != "" {
				sections = append(sections, "# Language-Specific Review Rules\n\n"+rel)
			}
		}
	}
	if len(d.Frameworks) > 0 {
		if c := loadSkillFile("frameworks"); c != "" {
			if rel := extractSections(c, d.Frameworks); rel != "" {
				sections = append(sections, "# Framework-Specific Review Rules\n\n"+rel)
			}
		}
	}
	if len(d.DevOps) > 0 {
		if c := loadSkillFile("devops"); c != "" {
			if rel := extractSections(c, d.DevOps); rel != "" {
				sections = append(sections, "# DevOps Review Rules\n\n"+rel)
			}
		}
	}
	for _, dname := range d.Domains {
		if dd, ok := domainIndicators[dname]; ok {
			if c := loadSkillFile(dd.skillFile); c != "" {
				sections = append(sections, c)
			}
		}
	}

	if len(sections) == 0 {
		return ""
	}

	var loaded []string
	loaded = append(loaded, d.Always...)
	loaded = append(loaded, d.Languages...)
	loaded = append(loaded, d.Frameworks...)
	loaded = append(loaded, d.DevOps...)
	for _, dname := range d.Domains {
		if dd, ok := domainIndicators[dname]; ok {
			loaded = append(loaded, dd.skillFile)
		}
	}

	header := "---\n\n# REVIEW SKILLS (auto-loaded based on detected files)\n\n"
	header += "**Skills loaded**: " + strings.Join(loaded, ", ") + "\n\n"
	return header + strings.Join(sections, "\n\n---\n\n")
}

// ── Helpers ─────────────────────────────────────────────────────────────────

var sectionSplitter = regexp.MustCompile(`(?m)^## `)

func extractSections(content string, keys []string) string {
	headerRe := regexp.MustCompile(`(?m)^## (.+)$`)
	allMatches := headerRe.FindAllStringIndex(content, -1)

	var matched []string
	for i, idx := range allMatches {
		end := len(content)
		if i+1 < len(allMatches) {
			end = allMatches[i+1][0]
		}
		section := content[idx[0]:end]
		headerMatch := headerRe.FindStringSubmatch(section)
		if len(headerMatch) < 2 {
			continue
		}
		headerLower := strings.ToLower(headerMatch[1])
		for _, key := range keys {
			if strings.Contains(headerLower, strings.ToLower(key)) {
				matched = append(matched, strings.TrimSpace(section))
				break
			}
		}
	}
	return strings.Join(matched, "\n\n")
}

func sortedKeys(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
