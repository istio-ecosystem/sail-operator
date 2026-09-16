# API Conventions — sail-operator

The authoritative API guidance lives in `AGENTS.md` and the domain knowledge files below. Read those before adding or modifying API types.

## Primary references

- **AGENTS.md § Code Style and Conventions** — API changes require a SEP (Sail Enhancement Proposal) first
- **AGENTS.md § Adding New API Fields** — step-by-step workflow (modify types → `make gen` → update controllers → add tests → SEP)
- **[API Types and CRDs](.agents/knowledge/domain-knowledge-api-types.md)** — CRD structure, validation rules, status conditions, resource relationships
- **[Version Management](.agents/knowledge/domain-knowledge-version-management.md)** — version compatibility and upgrade strategy when API changes span versions
