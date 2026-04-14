from __future__ import annotations

"""
ReviewIQ Skill Loader

Auto-detects languages, frameworks, and infrastructure from changed files,
then loads only the relevant skill modules into the system prompt.
This keeps token usage low — a Python/Django PR won't load React or Helm skills.

Skills are loaded from:
  1. .pr-review/skills/ (repo-level, customizable)
  2. Package templates (fallback defaults)
"""

import re
from pathlib import Path


# ── File Extension → Skill Mapping ───────────────────────────────────────────

LANGUAGE_MAP = {
    # Python
    ".py": "python", ".pyx": "python", ".pyi": "python",
    # Java / JVM
    ".java": "java", ".kt": "java", ".kts": "java", ".scala": "java", ".groovy": "java",
    # Go
    ".go": "golang",
    # TypeScript / JavaScript
    ".ts": "typescript", ".tsx": "typescript", ".js": "typescript", ".jsx": "typescript",
    ".mjs": "typescript", ".cjs": "typescript",
    # C / C++
    ".c": "cpp", ".cpp": "cpp", ".cc": "cpp", ".cxx": "cpp", ".h": "cpp",
    ".hpp": "cpp", ".hxx": "cpp",
    # Rust
    ".rs": "rust",
    # C#
    ".cs": "csharp",
    # Ruby
    ".rb": "ruby", ".erb": "ruby",
    # PHP
    ".php": "php",
    # Swift
    ".swift": "swift",
    # Kotlin (Android)
    ".kt": "kotlin",
    # SQL
    ".sql": "sql",
    # Shell
    ".sh": "shell", ".bash": "shell", ".zsh": "shell",
    # COBOL / Legacy
    ".cob": "legacy", ".cbl": "legacy", ".cpy": "legacy",
    # Fortran
    ".f": "legacy", ".f90": "legacy", ".f95": "legacy",
    # ABAP
    ".abap": "legacy",
}

FRAMEWORK_INDICATORS = {
    # Python frameworks
    "django": {"files": ["manage.py", "settings.py", "wsgi.py", "asgi.py"],
               "imports": ["django", "from django"]},
    "fastapi": {"files": [],
                "imports": ["fastapi", "from fastapi"]},
    "flask": {"files": [],
              "imports": ["flask", "from flask"]},
    "celery": {"files": ["celeryconfig.py"],
               "imports": ["celery", "from celery"]},
    "sqlalchemy": {"files": [],
                   "imports": ["sqlalchemy", "from sqlalchemy"]},
    # JS/TS frameworks
    "react": {"files": [],
              "imports": ["react", "from react", "'react'", "\"react\""]},
    "nextjs": {"files": ["next.config.js", "next.config.ts", "next.config.mjs"],
               "imports": ["next/", "from next"]},
    "express": {"files": [],
                "imports": ["express", "from express"]},
    "nestjs": {"files": [],
               "imports": ["@nestjs/"]},
    "vue": {"files": [],
            "imports": ["vue", "from vue"]},
    "angular": {"files": ["angular.json"],
                "imports": ["@angular/"]},
    # Java frameworks
    "spring": {"files": ["pom.xml", "build.gradle"],
               "imports": ["org.springframework", "spring-boot"]},
    # Ruby frameworks
    "rails": {"files": ["Gemfile", "Rakefile", "config/routes.rb"],
              "imports": ["rails", "activerecord"]},
    # .NET
    "dotnet": {"files": [],
               "imports": ["Microsoft.AspNetCore", "System.Linq", "Microsoft.EntityFrameworkCore"]},
}

DEVOPS_INDICATORS = {
    "docker": {
        "files": ["Dockerfile", "docker-compose.yml", "docker-compose.yaml",
                  ".dockerignore", "docker-compose.override.yml"],
        "extensions": [],
    },
    "kubernetes": {
        "files": [],
        "extensions": [".yaml", ".yml"],
        "content_patterns": ["apiVersion:", "kind: Deployment", "kind: Service",
                            "kind: Pod", "kind: ConfigMap", "kind: Ingress",
                            "kind: StatefulSet", "kind: DaemonSet"],
    },
    "helm": {
        "files": ["Chart.yaml", "Chart.yml", "values.yaml", "values.yml"],
        "extensions": [".tpl"],
        "dir_patterns": ["charts/", "templates/"],
    },
    "terraform": {
        "files": [],
        "extensions": [".tf", ".tfvars"],
    },
    "cicd": {
        "files": [".github/workflows/", ".gitlab-ci.yml", "Jenkinsfile",
                  ".circleci/config.yml", ".travis.yml", "azure-pipelines.yml",
                  "bitbucket-pipelines.yml"],
        "extensions": [],
    },
    "ansible": {
        "files": ["playbook.yml", "ansible.cfg", "inventory"],
        "extensions": [],
        "dir_patterns": ["roles/", "playbooks/"],
    },
}

# ── Always-loaded skills ─────────────────────────────────────────────────────

ALWAYS_LOAD = ["commandments", "security", "scalability", "stability", "maintainability", "performance"]

FINTECH_INDICATORS = {
    "imports": [
        "stripe", "razorpay", "paypal", "braintree", "adyen", "square",
        "plaid", "dwolla", "paytm", "phonepe", "cashfree", "payu",
        "payment", "checkout", "billing", "invoice", "subscription",
        "ledger", "accounting", "journal_entry", "double_entry",
        "loan", "emi", "amortization", "disbursement", "repayment",
        "interest_rate", "apr", "prepayment", "moratorium",
        "insurance", "policy", "premium", "claim", "underwriting",
        "coverage", "endorsement", "actuary",
        "kyc", "aml", "pci", "compliance", "sanctions",
        "bank_account", "ifsc", "iban", "swift", "ach", "nach",
        "credit_score", "bureau", "cibil", "experian", "equifax",
        "wallet", "upi", "neft", "rtgs", "imps",
    ],
    "files": [
        "payment", "checkout", "billing", "invoice", "subscription",
        "loan", "emi", "lending", "disburs", "repay", "collect",
        "insurance", "policy", "claim", "premium", "underwrit",
        "ledger", "journal", "accounting", "reconcil",
        "kyc", "aml", "compliance", "sanction",
        "wallet", "transfer", "payout", "refund", "settlement",
    ],
}

INDIA_REGULATORY_INDICATORS = {
    "imports": [
        "upi", "npci", "nach", "ecs", "neft", "rtgs", "imps",
        "aadhaar", "uidai", "digilocker", "ckyc", "ekyc", "e_kyc",
        "ifsc", "rbi", "nbfc", "cersai", "gst", "gstin",
        "razorpay", "paytm", "phonepe", "cashfree", "payu", "juspay",
        "account_aggregator", "fip", "fiu",
    ],
    "files": [
        "upi", "nach", "ecs", "neft", "rtgs", "imps",
        "aadhaar", "ekyc", "e_kyc", "ckyc", "digilocker",
        "nbfc", "rbi", "cersai", "gst", "mandate",
    ],
}

CREDIT_BUREAU_INDICATORS = {
    "imports": [
        "cibil", "transunion", "experian", "equifax", "crif",
        "credit_score", "credit_report", "bureau", "credit_bureau",
        "credit_check", "credit_pull", "hard_inquiry", "soft_inquiry",
    ],
    "files": [
        "bureau", "cibil", "credit_score", "credit_report", "credit_check",
    ],
}

FRAUD_INDICATORS = {
    "imports": [
        "fraud", "risk_score", "risk_engine", "velocity_check",
        "device_fingerprint", "fingerprint", "anti_fraud",
        "suspicious", "aml", "money_laundering", "sanctions",
        "chargeback", "dispute",
    ],
    "files": [
        "fraud", "risk_engine", "risk_score", "anti_fraud",
        "velocity", "device_fingerprint", "chargeback", "dispute",
    ],
}

NOTIFICATION_INDICATORS = {
    "imports": [
        "sms", "twilio", "msg91", "kaleyra", "gupshup",
        "sendgrid", "ses", "mailgun", "postmark",
        "fcm", "apns", "push_notification", "firebase_messaging",
        "whatsapp", "whatsapp_business",
        "dlt", "trai", "dnd",
    ],
    "files": [
        "notification", "sms", "email_service", "push",
        "whatsapp", "communication", "alert_service",
    ],
}

FINANCIAL_MICROSERVICES_INDICATORS = {
    "imports": [
        "saga", "compensat", "outbox", "event_sourcing",
        "distributed_transaction", "two_phase",
        "circuit_breaker", "bulkhead",
        "kafka", "rabbitmq", "celery", "sidekiq",
        "ledger_service", "payment_service", "settlement_service",
    ],
    "files": [
        "saga", "orchestrat", "compensat", "outbox",
        "event_store", "ledger_service", "settlement",
    ],
}

DATA_PRIVACY_INDICATORS = {
    "imports": [
        "gdpr", "ccpa", "dpdp", "privacy", "consent",
        "data_subject", "data_principal", "erasure", "anonymiz",
        "pseudonymiz", "pii", "personal_data",
        "right_to_forget", "data_protection",
    ],
    "files": [
        "privacy", "consent", "gdpr", "ccpa", "dpdp",
        "anonymiz", "pseudonymiz", "pii", "data_protection",
        "data_deletion", "data_export",
    ],
}

# Map of domain indicator name → (indicators_dict, skill_file_name)
DOMAIN_INDICATORS = {
    "fintech": (FINTECH_INDICATORS, "fintech"),
    "india_regulatory": (INDIA_REGULATORY_INDICATORS, "india-regulatory"),
    "credit_bureau": (CREDIT_BUREAU_INDICATORS, "credit-bureau"),
    "fraud": (FRAUD_INDICATORS, "fraud"),
    "notifications": (NOTIFICATION_INDICATORS, "notifications"),
    "financial_microservices": (FINANCIAL_MICROSERVICES_INDICATORS, "financial-microservices"),
    "data_privacy": (DATA_PRIVACY_INDICATORS, "data-privacy"),
}


# ── Detection Engine ─────────────────────────────────────────────────────────

def detect_skills(changed_files: list[str], file_contents: str = "") -> dict:
    """
    Detect which skills are relevant based on changed files and their contents.

    Returns:
        {
            "languages": ["python", "typescript"],
            "frameworks": ["django", "react"],
            "devops": ["docker", "kubernetes", "helm"],
            "always": ["commandments", "security", "scalability", "stability"],
        }
    """
    result = {
        "languages": set(),
        "frameworks": set(),
        "devops": set(),
        "domains": set(),
        "always": ALWAYS_LOAD,
    }

    content_lower = file_contents.lower()

    for filepath in changed_files:
        path = Path(filepath)
        ext = path.suffix.lower()
        name = path.name.lower()

        # Language detection
        if ext in LANGUAGE_MAP:
            result["languages"].add(LANGUAGE_MAP[ext])

        # DevOps detection by filename
        for devops_type, indicators in DEVOPS_INDICATORS.items():
            for pattern in indicators.get("files", []):
                if name == pattern.lower() or filepath.lower().endswith(pattern.lower()):
                    result["devops"].add(devops_type)
            for dev_ext in indicators.get("extensions", []):
                if ext == dev_ext:
                    result["devops"].add(devops_type)
            for dir_pat in indicators.get("dir_patterns", []):
                if dir_pat.lower() in filepath.lower():
                    result["devops"].add(devops_type)

        # Framework detection by filename
        for framework, indicators in FRAMEWORK_INDICATORS.items():
            for pattern in indicators.get("files", []):
                if name == pattern.lower() or filepath.lower().endswith(pattern.lower()):
                    result["frameworks"].add(framework)

    # Content-based detection (imports, patterns)
    if file_contents:
        # Framework detection by imports
        for framework, indicators in FRAMEWORK_INDICATORS.items():
            for imp in indicators.get("imports", []):
                if imp.lower() in content_lower:
                    result["frameworks"].add(framework)

        # Kubernetes content patterns
        k8s_indicators = DEVOPS_INDICATORS.get("kubernetes", {})
        for pattern in k8s_indicators.get("content_patterns", []):
            if pattern.lower() in content_lower:
                result["devops"].add("kubernetes")

    # Domain detection (fintech, fraud, privacy, etc.)
    for domain_name, (indicators, _) in DOMAIN_INDICATORS.items():
        detected = False
        if file_contents:
            for pattern in indicators.get("imports", []):
                if pattern.lower() in content_lower:
                    detected = True
                    break
        if not detected:
            for pattern in indicators.get("files", []):
                for filepath in changed_files:
                    if pattern.lower() in filepath.lower():
                        detected = True
                        break
                if detected:
                    break
        if detected:
            result["domains"].add(domain_name)

    # Convert sets to sorted lists for deterministic ordering
    result["languages"] = sorted(result["languages"])
    result["frameworks"] = sorted(result["frameworks"])
    result["devops"] = sorted(result["devops"])
    result["domains"] = sorted(result["domains"])

    return result


# ── Skill File Loading ───────────────────────────────────────────────────────

def _skill_dirs() -> list[Path]:
    """Return skill directories in priority order (repo first, package fallback)."""
    dirs = []
    repo_skills = Path(".pr-review/skills")
    if repo_skills.exists():
        dirs.append(repo_skills)
    pkg_skills = Path(__file__).parent / "templates" / "skills"
    if pkg_skills.exists():
        dirs.append(pkg_skills)
    return dirs


def _load_skill_file(name: str) -> str | None:
    """Load a skill file by name from the first available directory."""
    for skill_dir in _skill_dirs():
        path = skill_dir / f"{name}.md"
        if path.exists():
            return path.read_text()
    return None


def load_skills(detected: dict) -> str:
    """
    Load all relevant skill files and compose them into a single prompt section.

    Only loads what's needed — a Python/Django PR won't load React or Helm skills.
    """
    sections = []

    # Always-loaded skills
    for skill_name in detected["always"]:
        content = _load_skill_file(skill_name)
        if content:
            sections.append(content)

    # Language skills
    if detected["languages"]:
        content = _load_skill_file("languages")
        if content:
            # Extract only relevant language sections
            relevant = _extract_sections(content, detected["languages"])
            if relevant:
                sections.append(f"# Language-Specific Review Rules\n\n{relevant}")

    # Framework skills
    if detected["frameworks"]:
        content = _load_skill_file("frameworks")
        if content:
            relevant = _extract_sections(content, detected["frameworks"])
            if relevant:
                sections.append(f"# Framework-Specific Review Rules\n\n{relevant}")

    # DevOps skills
    if detected["devops"]:
        content = _load_skill_file("devops")
        if content:
            relevant = _extract_sections(content, detected["devops"])
            if relevant:
                sections.append(f"# DevOps Review Rules\n\n{relevant}")

    # Domain-specific skills (fintech, fraud, privacy, etc.)
    for domain_name in detected.get("domains", []):
        _, skill_file = DOMAIN_INDICATORS.get(domain_name, (None, None))
        if skill_file:
            content = _load_skill_file(skill_file)
            if content:
                sections.append(content)

    if not sections:
        return ""

    header = "---\n\n# REVIEW SKILLS (auto-loaded based on detected files)\n\n"
    skills_loaded = []
    skills_loaded.extend(detected["always"])
    skills_loaded.extend(detected["languages"])
    skills_loaded.extend(detected["frameworks"])
    skills_loaded.extend(detected["devops"])
    for domain_name in detected.get("domains", []):
        _, skill_file = DOMAIN_INDICATORS.get(domain_name, (None, None))
        if skill_file:
            skills_loaded.append(skill_file)

    header += f"**Skills loaded**: {', '.join(skills_loaded)}\n\n"
    return header + "\n\n---\n\n".join(sections)


def _extract_sections(content: str, keys: list[str]) -> str:
    """
    Extract only the sections matching the given keys from a skill file.

    Sections are delimited by ## headers. A section matches if any key
    appears in the header (case-insensitive).
    """
    sections = re.split(r"(?=^## )", content, flags=re.MULTILINE)
    matched = []

    for section in sections:
        header_match = re.match(r"^## (.+)$", section, re.MULTILINE)
        if not header_match:
            continue
        header = header_match.group(1).lower()
        for key in keys:
            if key.lower() in header:
                matched.append(section.strip())
                break

    return "\n\n".join(matched)
