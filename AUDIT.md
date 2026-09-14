# Job board audit — September 14, 2026

Checked every configured adapter against its public live feed. All 114 boards returned non-empty results without errors: 15,146 postings initially, 15,145 on verification (live listings changed between reads). Matching totals increased from 122 to 130, including the previously added Cohere/Harvey review alerts. These counts are filter matches, not confirmed new-grad vacancies.

## Confirmed gaps fixed

| Company | Previously missed posting / signal |
|---|---|
| Justworks | Associate Software Engineer, Expenses |
| PointClickCare | Canada- Jr Software Engineer (SRE) |
| D2L | Software Test Developer - New Graduate |
| AlayaCare | Junior Fullstack Developer (Python) |
| Trulioo | Junior Software Engineer |
| Vestwell | Associate, Software Engineer; description accepts classwork, internships or 0–2 years |
| WHOOP | Android Engineer I |
| Replit | Cohort 0; structured employment type is Intern, department Engineering |

The title rules require a software title before junior/associate/graduate/I/1 counts as early-career. Explicit seniority and location exclusions still run first. Adapters now preserve Ashby/Lever team and internship metadata, all Greenhouse departments, and Lever secondary locations. Regression tests cover these cases and unwanted business/senior/foreign matches.

## Limits

This is an adapter and filter audit, plus targeted description checks; it is not a manual review of every description. Outside Cohere/Harvey, plain engineering titles without early-career metadata still do not match. The bot does not inspect descriptions for graduate eligibility. Snap Level 3 remains excluded under the existing repository policy because the level is not consistently entry-level. Free-text locations and broad engineering departments remain heuristic, and boards can later migrate or omit postings. First runs seed silently; previously unseen matches can alert after deployment with existing state. Nothing was deployed and no Discord messages were sent during this audit.

## Board-by-board verification

Zero matches means the current filter found none, not that the company never hires graduates. Counts include internships and the Cohere/Harvey review safety net.

| Company | Adapter / slug | Listed jobs | Matches |
|---|---|---:|---:|
| Ramp | ashby / ramp | 145 | 0 |
| Notion | ashby / notion | 126 | 5 |
| Harvey | ashby / harvey | 322 | 4 |
| Stripe | greenhouse / stripe | 630 | 4 |
| Pinterest | greenhouse / pinterest | 182 | 0 |
| Figma | greenhouse / figma | 154 | 1 |
| Spotify | lever / spotify | 70 | 0 |
| Rippling | rippling / rippling | 654 | 1 |
| Brex | greenhouse / brex | 269 | 0 |
| Coinbase | greenhouse / coinbase | 217 | 2 |
| Robinhood | greenhouse / robinhood | 151 | 8 |
| Asana | greenhouse / asana | 101 | 0 |
| Peloton | greenhouse / peloton | 54 | 0 |
| Squarespace | greenhouse / squarespace | 29 | 0 |
| Betterment | greenhouse / betterment | 25 | 0 |
| Cockroach | greenhouse / cockroachlabs | 21 | 0 |
| Airtable | greenhouse / airtable | 16 | 0 |
| OpenAI | ashby / openai | 792 | 0 |
| Sierra | ashby / sierra | 209 | 2 |
| Cursor | ashby / cursor | 123 | 1 |
| Vanta | ashby / vanta | 104 | 0 |
| ClickUp | ashby / clickup | 62 | 0 |
| Linear | ashby / linear | 30 | 0 |
| Airbnb | greenhouse / airbnb | 162 | 0 |
| Block | greenhouse / block | 207 | 0 |
| Cloudflare | greenhouse / cloudflare | 356 | 0 |
| Databricks | greenhouse / databricks | 886 | 1 |
| Discord | greenhouse / discord | 46 | 0 |
| Dropbox | greenhouse / dropbox | 41 | 1 |
| Lyft | greenhouse / lyft | 181 | 7 |
| Twitch | greenhouse / twitch | 55 | 5 |
| Plaid | ashby / plaid | 111 | 0 |
| Anthropic | greenhouse / anthropic | 597 | 0 |
| Cohere | ashby / cohere | 144 | 36 |
| Perplexity | ashby / perplexity | 115 | 0 |
| Cognition | ashby / cognition | 94 | 0 |
| ElevenLabs | ashby / elevenlabs | 246 | 0 |
| Baseten | ashby / baseten | 93 | 0 |
| Modal | ashby / modal | 31 | 1 |
| LangChain | ashby / langchain | 108 | 2 |
| Decagon | ashby / decagon | 141 | 0 |
| Abridge | ashby / abridge | 40 | 1 |
| Vercel | greenhouse / vercel | 86 | 2 |
| MongoDB | greenhouse / mongodb | 405 | 0 |
| Supabase | ashby / supabase | 60 | 0 |
| Temporal | ashby / temporal | 70 | 0 |
| Sentry | ashby / sentry | 43 | 0 |
| Confluent | ashby / confluent | 21 | 0 |
| Hex | ashby / hex | 33 | 0 |
| Affirm | greenhouse / affirm | 207 | 1 |
| Chime | greenhouse / chime | 69 | 0 |
| Braze | greenhouse / braze | 279 | 0 |
| Justworks | greenhouse / justworks | 96 | 1 |
| Zocdoc | greenhouse / zocdoc | 52 | 0 |
| Alloy | greenhouse / alloy | 25 | 0 |
| Lithic | greenhouse / lithic | 10 | 0 |
| Attentive | greenhouse / attentive | 34 | 0 |
| Yext | greenhouse / yext | 20 | 0 |
| FlatironHealth | greenhouse / flatironhealth | 28 | 0 |
| Middesk | ashby / middesk | 17 | 0 |
| ModernTreasury | ashby / moderntreasury | 9 | 0 |
| Ro | lever / ro | 46 | 0 |
| DoorDash | greenhouse / doordashusa | 455 | 0 |
| Kalshi | ashby / kalshi | 40 | 0 |
| Polymarket | ashby / polymarket | 77 | 0 |
| Bumble | ashby / bumbleinc | 25 | 0 |
| Snap | snap / careers.snap.com | 175 | 0 |
| StackAdapt | greenhouse / stackadapt | 73 | 0 |
| Wealthsimple | ashby / wealthsimple | 56 | 2 |
| 1Password | ashby / 1password | 60 | 0 |
| PointClickCare | lever / pointclickcare | 79 | 1 |
| Waabi | lever / waabi | 86 | 0 |
| Lightspeed | ashby / lightspeedhq | 142 | 0 |
| Jobber | ashby / jobber | 37 | 0 |
| D2L | greenhouse / d2l | 31 | 4 |
| AlayaCare | greenhouse / alayacare | 21 | 1 |
| Hopper | ashby / hopper | 36 | 0 |
| Jane | ashby / jane | 27 | 0 |
| Instacart | greenhouse / instacart | 104 | 0 |
| Miovision | ashby / miovision | 15 | 0 |
| Benevity | ashby / benevity | 10 | 0 |
| Trulioo | ashby / trulioo | 49 | 1 |
| Mercury | greenhouse / mercury | 61 | 0 |
| CoreWeave | greenhouse / coreweave | 294 | 0 |
| Clio | workday / clio/wd3/ClioCareerSite | 123 | 0 |
| ClearStreet | greenhouse / clearstreet | 31 | 0 |
| Rho | ashby / rho | 52 | 0 |
| Flex | greenhouse / flex | 42 | 0 |
| Gemini | greenhouse / gemini | 32 | 0 |
| Fireblocks | greenhouse / fireblocks | 72 | 0 |
| Vestwell | greenhouse / vestwell | 18 | 1 |
| Melio | greenhouse / melio | 24 | 0 |
| Pagaya | greenhouse / pagaya | 11 | 0 |
| Carta | greenhouse / carta | 64 | 0 |
| AlphaSense | greenhouse / alphasense | 221 | 0 |
| Verkada | greenhouse / verkada | 293 | 15 |
| ScaleAI | greenhouse / scaleai | 228 | 2 |
| Rubrik | greenhouse / rubrik | 138 | 0 |
| Crusoe | ashby / crusoe | 370 | 1 |
| Whoop | ashby / whoop | 154 | 2 |
| Nuro | greenhouse / nuro | 105 | 2 |
| Samsara | greenhouse / samsara | 246 | 1 |
| Benchling | ashby / benchling | 51 | 0 |
| Cerebras | ashby / cerebras | 111 | 2 |
| Replit | ashby / replit | 73 | 2 |
| Neuralink | greenhouse / neuralink | 79 | 7 |
| Profound | ashby / profound | 74 | 0 |
| Via | greenhouse / via | 158 | 0 |
| Gusto | greenhouse / gusto | 96 | 0 |
| Reddit | greenhouse / reddit | 148 | 0 |
| SoFi | greenhouse / sofi | 58 | 0 |
| Duolingo | greenhouse / duolingo | 80 | 0 |
| Faire | greenhouse / faire | 67 | 0 |
| GitLab | greenhouse / gitlab | 223 | 0 |

