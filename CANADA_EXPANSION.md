# Canadian watchlist expansion — November 2, 2026

Added 12 employers to `companies.json`, increasing the watchlist from 449 to 461. Research prioritizes Toronto, Metro Vancouver and remote work explicitly available in Canada for a December 2026 graduate. Existing US/Canada coverage remains enabled. The bot filters early-career signals, not graduation-date eligibility or years of experience in descriptions; review those before applying.

All 12 boards returned non-empty results through the production adapters in a read-only live audit. A second audit hit a transient 20-second timeout on Veeva; an isolated retry fetched all 889 postings successfully in about seven seconds. That is a real latency limit to monitor, not a guaranteed scan-time improvement. This did not send Discord messages or alter seen state. The changes have not been deployed.

| Added employer | Verified feed | Why monitor it / current limits |
|---|---|---|
| Quora | [Ashby](https://jobs.ashbyhq.com/quora) | Three live new-grad engineering roles with Canada as a secondary location. Full-stack and ML-platform postings accept 2025/2026 graduates; the ML role specifies an early-2027 start. |
| Veeva Systems | [Lever](https://jobs.lever.co/veeva) | Live Associate Software Engineer in Test role in Toronto welcomes graduates. The posting requires a 3.0+ GPA and four office days per week during the first two years. Its separate engineering-development program is also worth monitoring; the advertised developer role was absent from this feed snapshot. |
| Konrad | [Greenhouse](https://job-boards.greenhouse.io/konradgroup) | Live entry-level software and mobile developer roles in Toronto. Degree completion is requested; confirm a January start with recruiting. |
| Evismart | [Greenhouse](https://job-boards.greenhouse.io/evismart) | Live junior full-stack engineering role in Vancouver. On-site and asks for 1–2 years of professional experience: a stretch opportunity, not a confirmed zero-experience opening. |
| Harbor | [Greenhouse](https://job-boards.greenhouse.io/harborglobal) | Live junior developer role supports remote Canada, encourages recent graduates and lists 0–2 years of experience. Occasional travel may be required. |
| Canonical | [Greenhouse](https://job-boards.greenhouse.io/canonical) | Live graduate software roles with worldwide or Americas remote locations. Confirm Canadian employment arrangements and start timing for the individual role. The old `canonicaljobs` feed returns 404; use `canonical`. |
| IXL Learning | [Greenhouse](https://job-boards.greenhouse.io/ixllearning) | Proven Toronto new-grad developer hiring, with a live board and US new-grad engineering roles. No Toronto new-grad role was present in this snapshot; monitor for future intake. |
| AtoB | [Ashby](https://jobs.ashbyhq.com/atob) | Previously advertised a six-month new-grad internship-to-hire program including Toronto and Vancouver. That listing is closed and absent from the current feed; monitoring only. |
| Global Relay | [Greenhouse](https://job-boards.greenhouse.io/globalrelay) | Vancouver engineering employer with a live co-op expression-of-interest listing. No current confirmed graduate SWE match; future monitoring. Co-op eligibility may require continued enrolment. |
| Later | [Greenhouse](https://job-boards.greenhouse.io/later) | Vancouver engineering team and live early-career/co-op hiring. Current co-ops may require returning to school, so they are not assumed suitable after graduation. |
| Fullscript | [Lever](https://jobs.lever.co/fullscript) | Canadian engineering employer with remote-Canada hiring and a live developer co-op opportunity in Ottawa. No confirmed full-time graduate SWE opening in the target locations at this check; monitor future intake and verify student eligibility for co-ops. |
| Toast | [Greenhouse](https://job-boards.greenhouse.io/toast) | Canadian remote/hybrid engineering hiring and previously advertised Software Engineer I positions. No current junior Canadian engineering role was present in this snapshot. |

## Current application shortlist

These were present in the actual feeds during verification. A feed listing does not confirm a January 2027 start; ask the employer where the posting does not specify one.

| Priority | Role | Location | Fit and checks |
|---|---|---|---|
| High | [Quora — Full Stack Software Engineer, New Grad](https://jobs.ashbyhq.com/quora/5494e6a1-a973-46af-b9f3-57208eb91b52) | Remote Canada | Explicit 2025/2026 graduate eligibility; Python/React. Requires availability 9am–3pm Pacific. |
| High | [Quora — Software Engineer New Grad, ML Platform](https://jobs.ashbyhq.com/quora/cf34f80e-fe5c-454d-bc9a-4c59993ffda0) | Remote Canada | Explicit 2025/2026 graduate eligibility; Python/Go/C++, infrastructure. No prior ML-infrastructure experience required. |
| High, if ML projects fit | [Quora — Machine Learning Engineer, New Grad](https://jobs.ashbyhq.com/quora/40659fe7-545e-4b08-b6d7-2567b18b9ed3) | Remote Canada | Early-2027 full-time start; asks for ML foundations and LLM coursework, research or projects. |
| High | [Harbor — Junior Developer, Finance Systems](https://job-boards.greenhouse.io/harborglobal/jobs/5238900007) | Remote Canada | Recent graduates encouraged; 0–2 years; C#/.NET and integrations. |
| High | [Konrad — Software Developer, Entry Level](https://www.konrad.com/careers/job/software-developer-entry-level_5893240003) | Toronto | Entry-level web development; degree requested. Confirm post-December start. |
| High | [Konrad — Mobile Developer, Entry Level](https://www.konrad.com/careers/job/7976082003?gh_jid=7976082003) | Toronto | Entry-level mobile development. Confirm start timing and platform requirements. |
| High, if testing interests you | [Veeva — Associate Software Engineer in Test](https://jobs.lever.co/veeva/fecaef42-b5c1-4fb2-b819-058171174ed1) | Toronto | Graduates welcome; Java, automation; GPA and in-office requirements above. |
| Consider | [Canonical — Graduate Software Engineer](https://job-boards.greenhouse.io/canonical/jobs/8249005) | Worldwide remote | Graduate open-source/Linux track. Verify Canadian employment and start date. |
| Stretch | [Evismart — Junior Full Stack Engineer](https://job-boards.greenhouse.io/evismart/jobs/4457881009) | Vancouver, on-site | Asks for 1–2 years; JavaScript/TypeScript frontend plus backend experience. |

Already watched and still present: [Cerebras new-grad software](https://jobs.ashbyhq.com/cerebras/99c289fa-8fc6-49f7-b7e8-78ac4e9d99ac) and [DevOps](https://jobs.ashbyhq.com/cerebras/40e0d3ee-8f0a-4b19-9bf9-79410b1c7735) in Toronto; [Stripe new-grad SWE](https://stripe.com/jobs/search?gh_jid=8157838) in Toronto; [Affirm SWE I, Fullstack](https://job-boards.greenhouse.io/affirm/jobs/8017342003) and [Frontend](https://job-boards.greenhouse.io/affirm/jobs/7985907003) in remote Canada; [PolicyMe junior SWE](https://jobs.lever.co/policyme/30e81807-adb5-44a9-baad-4beb75913694), whose description limits remote work to Ontario.

Search-indexed junior/new-grad listings at Zip, Magical and 1Password were absent from their live feeds. They remain watched, but those indexed listings are not counted as current openings. Thoughtworks’ Toronto graduate opening was on a referrals-only board, so that board was not added.

## Useful openings outside the supported feeds

[Atlassian — Software Engineer, 2027 Graduate Canada](https://campus-globalcareers-atlassian.icims.com/jobs/26070/software-engineer%2C-2027-graduate-canada/job?mobile=true&needsRedirect=false) is a Metro Vancouver opportunity requiring graduation by August 2027. Its iCIMS board is not supported by this bot, so apply or set up alerts directly; it was not added as a pretend supported feed. Confirm December 2026 eligibility and work authorization with recruiting.

## Filter and verification notes

The expansion also fixes missed software titles: `Junior Developer`, `Jr. Developer`, `Graduate Developer`, and mobile/Android/iOS developer variants. Existing seniority and location exclusions still apply, and junior business-developer titles remain excluded. Costa Rica was added to the foreign-location exclusion after the new Konrad board exposed an out-of-scope internship. The bot still does not classify job descriptions or guarantee graduate eligibility.

The read-only audit accepts `JOBWATCH_AUDIT_COMPANIES=/path/to/companies.json` for a selected list and includes `MatchingJobs` alongside counts. To repeat only these additions, extract the 12 names above into a JSON company list using the same schema as `companies.json`, then run:

```sh
JOBWATCH_AUDIT=/tmp/canada-audit.json \
JOBWATCH_AUDIT_COMPANIES=/tmp/canada-companies.json \
go test -run TestLiveBoardAudit -v -count=1 .
```

The first production run with no state seeds silently. With existing state, unseen matching postings from these companies can alert, subject to the per-cycle cap. No applications have been submitted.
