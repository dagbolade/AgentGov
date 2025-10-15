# Paper Proposal: Governance-by-Design: A WASM-based Human-in-the-Loop Sidecar for Safe AI Tooling

## Objective and Significance

As AI agents become increasingly autonomous and interact with external tools and APIs, the need for runtime governance mechanisms that balance agent autonomy with human oversight becomes critical. Current approaches to AI safety rely on pre-deployment testing, model fine-tuning, or post-hoc analysis, leaving a gap in real-time, enforceable policy controls that can adapt to organizational requirements while maintaining auditability.

This paper presents AgentGov, a governance-by-design sidecar architecture that intercepts AI agent tool calls, evaluates them against pluggable WebAssembly (WASM) policies, maintains an immutable audit trail, and integrates a human-in-the-loop approval workflow for high-risk operations. The significance for HCI lies in demonstrating how technical infrastructure can support transparent, auditable human-AI collaboration while preserving system performance and developer experience.

## Methods and Approach

### System Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         AI Agent                                 │
│                    (e.g., LangChain, AutoGPT)                   │
└───────────────────────────┬─────────────────────────────────────┘
                            │ Tool Call Request
                            │ (HTTP POST)
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│                   AgentGov Sidecar Proxy                         │
│                                                                   │
│  ┌─────────────────┐      ┌──────────────────┐                 │
│  │  WASM Policy    │      │  Approval Queue  │                 │
│  │    Engine       │◄────►│   (In-Memory)    │                 │
│  │                 │      │                  │                 │
│  │ • evaluate()    │      │ • Enqueue()      │                 │
│  │ • host funcs    │      │ • REST API       │                 │
│  │ • hot-reload    │      │ • Timeouts       │                 │
│  └────────┬────────┘      └────────┬─────────┘                 │
│           │                        │                            │
│           │                        │                            │
│           ▼                        ▼                            │
│  ┌─────────────────────────────────────────────┐               │
│  │         Audit Trail (SQLite)                │               │
│  │  • Immutable log of all decisions           │               │
│  │  • Policy evaluations + confidence          │               │
│  │  • Human approvals/rejections               │               │
│  │  • Request/response payloads                │               │
│  └─────────────────────────────────────────────┘               │
│                                                                   │
└───────────────────────────┬─────────────────────────────────────┘
                            │
        ┌───────────────────┼───────────────────┐
        │                   │                   │
        │ ALLOW             │ DENY              │ REQUIRES_APPROVAL
        ▼                   ▼                   ▼
┌──────────────┐    ┌──────────────┐    ┌──────────────────┐
│  Forward to  │    │    Return    │    │   Web UI for     │
│  Tool API    │    │    Error     │    │ Human Decision   │
│              │    │              │    │                  │
│ • GitHub API │    │ • 403/429    │    │ • Policy reason  │
│ • Jira API   │    │ • Audit log  │    │ • Risk context   │
│ • Shell Exec │    │              │    │ • Approve/Reject │
└──────────────┘    └──────────────┘    └──────────────────┘
        │                                        │
        │ Response                               │ Decision
        ▼                                        ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Return to AI Agent                          │
└─────────────────────────────────────────────────────────────────┘
```

AgentGov implements a transparent HTTP proxy that sits between AI agents and their tool endpoints. The core architecture consists of four integrated subsystems:

**1. WASM Policy Engine**: Policies are compiled from Rust to WASM modules exposing a standardized ABI (`evaluate`, `alloc`, `dealloc` functions). The host runtime (`policy.WASMEvaluator` in `internal/policy/evaluator.go`) provides sandboxed execution with memory isolation, supporting hot-reload without system restart. Host functions enable policies to log decisions and access environment context while maintaining security boundaries.

**2. Approval Queue System**: When policies flag operations as requiring human review, requests enter an in-memory queue (`approval.InMemoryQueue` in `internal/approval/queue.go`) with configurable timeout and priority handling. The queue exposes REST endpoints for approval/rejection with optional justification fields, enabling asynchronous human decision-making.

**3. Audit Infrastructure**: All policy evaluations, approvals, and tool executions generate immutable audit entries stored in SQLite with Write-Ahead Logging (`internal/audit/types.go` and `queries.go`). The audit schema captures policy decisions, confidence scores, human justifications, and full request/response payloads for forensic analysis.

**4. User Interface**: A web-based approval dashboard presents pending requests with policy reasoning, risk assessments, and contextual information to help human operators make informed decisions.

### Evaluation Methodology

Our evaluation combines quantitative performance benchmarking with qualitative usability assessment:

**Performance Analysis**: Microbenchmarks measure policy evaluation latency, memory overhead, cold-start times, and end-to-end proxy overhead under synthetic workloads. System-level throughput tests simulate concurrent agent operations with varying policy complexity.

**Usability Study**: Domain experts (N=8-10, including security engineers, compliance officers, and ML engineers) interact with the approval interface while processing realistic AI agent requests. We measure time-to-decision, decision confidence, and gather qualitative feedback on policy reason interpretability through think-aloud protocols and semi-structured interviews.

**Case Studies**: Three policy implementations demonstrate different governance patterns: (1) sensitive data detection using pattern matching, (2) rate limiting with temporal tracking, and (3) cost control with cumulative resource monitoring.

## Expected Results

| Metric | Expected Value | Measurement Context |
|--------|----------------|---------------------|
| Policy evaluation latency (P50) | 0.1–1 ms | Single policy call, warm cache |
| Policy evaluation latency (P95) | 2–5 ms | Complex multi-rule policies |
| Policy cold-start (load + compile) | 3–8 ms | First invocation after reload |
| Audit write latency | <1 ms | Async SQLite WAL mode |
| End-to-end proxy overhead (P95) | 10–50 ms | Including policy eval + forwarding |
| Memory per policy instance | 1–5 MB | WASM linear memory allocation |
| Approval queue decision time | 2–15 min (median) | Human-dependent, measured in study |
| False positive rate | 5–15% | Policy-specific, tunable thresholds |

We expect policies to introduce minimal latency overhead while providing meaningful safety guarantees. The human approval workflow will reveal tension between decision thoroughness and operational velocity, informing design guidelines for approval interfaces.

## Contributions and Implications

**For HCI Research**: This work contributes empirical evidence on designing human-AI control surfaces that balance automation with meaningful human agency. We identify design patterns for presenting machine-generated policy reasoning to human operators, addressing the challenge of trust calibration in high-stakes AI-assisted systems.

**For Technical Practice**: The WASM-based policy architecture provides a reusable pattern for implementing runtime governance in AI systems. The standardized ABI and host function design enable policy composition, testing, and gradual rollout. Open-source artifacts include reference policy implementations, test harnesses, and deployment guides.

**For AI Safety**: By demonstrating that governance mechanisms can operate with minimal performance penalty, we reduce barriers to adopting runtime safety controls. The audit trail architecture supports compliance requirements and incident investigation, while the human-in-the-loop system provides a fail-safe for high-consequence operations.

**Design Guidelines**: Based on implementation experience and user study findings, we will provide actionable guidelines for organizations implementing AI governance systems, covering policy expressiveness vs. complexity tradeoffs, approval workflow design, and audit data utilization.

The system is production-oriented, with containerized deployment, health monitoring, and graceful degradation handling. All code and documentation will be publicly available to support replication and extension by other researchers and practitioners.

## References

1. Amershi, S., Weld, D., Vorvoreanu, M., Fourney, A., Nushi, B., Collisson, P., Suh, J., Iqbal, S., Bennett, P. N., Inkpen, K., Teevan, J., Kikin-Gil, R., & Horvitz, E. (2019). Guidelines for Human-AI Interaction. *CHI Conference on Human Factors in Computing Systems*. https://doi.org/10.1145/3290605.3300233

2. Brundage, M., Avin, S., Wang, J., Belfield, H., Krueger, G., Hadfield, G., et al. (2020). Toward Trustworthy AI Development: Mechanisms for Supporting Verifiable Claims. https://arxiv.org/abs/2004.07213

3. Doshi-Velez, F., & Kim, B. (2017). Towards A Rigorous Science of Interpretable Machine Learning. https://arxiv.org/abs/1702.08608

4. Green, B., & Chen, Y. (2019). The Principles and Limits of Algorithm-in-the-Loop Decision Making. *Proceedings of the ACM on Human-Computer Interaction, 3*(CSCW), 1-24. https://doi.org/10.1145/3359152

5. Kulesza, T., Stumpf, S., Burnett, M., Yang, S., Kwan, I., & Wong, W. K. (2013). Too much, too little, or just right? Ways explanations impact end users' mental models. *IEEE Symposium on Visual Languages and Human Centric Computing*, 3-10. https://doi.org/10.1109/VLHCC.2013.6645235

6. Lyons, J. B., Wynne, K. T., Mahoney, S., & Roebke, M. A. (2019). Trust and Human-Machine Teaming: A Qualitative Study. *AAAI Spring Symposium on Intelligent Augmentation*. https://cdn.aaai.org/ocs/ws/ws0357/18020-80211-1-PB.pdf

7. OpenAI. (2023). GPT-4 System Card. https://cdn.openai.com/papers/gpt-4-system-card.pdf

8. Parasuraman, R., & Riley, V. (1997). Humans and Automation: Use, Misuse, Disuse, Abuse. *Human Factors, 39*(2), 230-253. https://doi.org/10.1518/001872097778543886

9. Raji, I. D., Smart, A., White, R. N., Mitchell, M., Gebru, T., Hutchinson, B., Smith-Loud, J., Theron, D., & Barnes, P. (2020). Closing the AI Accountability Gap: Defining an End-to-End Framework for Internal Algorithmic Auditing. *FAT* Conference on Fairness, Accountability, and Transparency*, 33-44. https://doi.org/10.1145/3351095.3372873

10. Shneiderman, B. (2020). Human-Centered Artificial Intelligence: Reliable, Safe & Trustworthy. *International Journal of Human-Computer Interaction, 36*(6), 495-504. https://doi.org/10.1080/10447318.2020.1741118

11. Vaughan, J. W., & Wallach, H. (2021). A Human-Centered Agenda for Intelligible Machine Learning. *Computers and Society: Modern Perspectives*. https://arxiv.org/abs/2101.11257

12. Weld, D. S., & Bansal, G. (2019). The Challenge of Crafting Intelligible Intelligence. *Communications of the ACM, 62*(6), 70-79. https://doi.org/10.1145/3282486

13. WebAssembly Community Group. (2023). WebAssembly Core Specification. https://www.w3.org/TR/wasm-core-2/

14. Xu, W., Dainoff, M. J., Ge, L., & Gao, Z. (2023). From Human-Computer Interaction to Human-AI Interaction: New Challenges and Opportunities for Enabling Human-Centered AI. *Applied Sciences, 13*(4), 2704. https://doi.org/10.3390/app13042704

15. Zhang, Y., Liao, Q. V., & Bellamy, R. K. E. (2020). Effect of Confidence and Explanation on Accuracy and Trust Calibration in AI-Assisted Decision Making. *FAT* Conference on Fairness, Accountability, and Transparency*, 295-305. https://doi.org/10.1145/3351095.3372852

---

**Word Count**: 798 words (excluding title, diagram, table, references, and this note)
