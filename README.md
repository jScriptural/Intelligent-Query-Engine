# Intelligent Query Engine

## Project Overview
This is a profile discovery engine designed to bridge the gap between structured data and human intent. Core feature Natural Language process- this allows recruiters and admins to query the database using natural conversational patterns alongside standard API parameters.

## Core Feature: The NLProcessor
The `NLProcessor` is a custom-built service utility that performs **intent extraction** using a combination of tokenization, keyword mapping, and regular expression pattern matching.

### 1. Tokenization & Keyword Mapping
The processor breaks down the input string into individual "fields" (tokens). It then performs a high-speed lookup against a `keywordsMap` to identify intent that isn't explicitly numeric.

* **Expansion Logic:** Keywords like `young` are expanded into multi-parameter filters (e.g., `min_age=16&max_age=24`).
* **Normalization:** All input is forced to lowercase to ensure "Males", "males", and "MALES" are treated identically.


### 2. Regex Intent Extraction (The Age Engine)
To handle the infinite ways humans describe age ranges, the processor employs non-greedy regex capturing:

| Human Phrase | Regex Pattern | Extraction |
| :--- | :--- | :--- |
| "Over 25" | `(above\|over)\s*?(\d+)` | `min_age=25` |
| "Under 40" | `(below\|under)\s*?(\d+)` | `max_age=40` |
| "Between 18 and 25" | `between\s*?(\d+)\s*?and\s*?(\d+)` | `min_age=18`, `max_age=25` |

### 3. Intent Merging & Conflict Resolution
The `NLProcessor` follows a **Last-In-Wins** priority model. If a user provides a structured query parameter (via URL) and a natural language query (via `q`), the values extracted from the `q` string will overwrite the URL parameters. 

**Flow:**
1. Raw `url.Values` are captured from the request.
2. `NLProcessor` identifies new filters from the search string.
3. Identified filters are injected into the existing `url.Values` using the `Set()` method.
4. The "Hydrated" query object is passed to the Database Store.



## System Architecture

### Service Layer (`internal/service`)
* **Validation**: Implements a strict whitelist for `sort_by` and `order` to prevent SQL injection.
* **Error Handling**: Implements `models.ErrUnInterpretable`. If the NLP string yields zero searchable tokens, the system rejects the request rather than guessing, ensuring high data accuracy.

### Store Layer (`internal/store`)
* **Dynamic SQL**: Uses `strings.Builder` to construct a `WHERE` clause dynamically based on the 10+ supported filters.
* **Pagination Metadata**: Returns both the result slice and the `total_count` from a single execution flow to support frontend "Page X of Y" calculations.

## API Specification

### List Profiles
`GET /api/profiles/search`

| Parameter | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `nl` | string | No | The natural language query string. |
| `page` | int | No | Default: 1. |
| `limit` | int | No | Default: 10. Max: 100. |
| `sort_by` | string | No | `age`, `created_at`, `gender_probability`. |

**Example Request:**
`GET /api/profiles/search?q=female teenagers between 15 and 19&sort_by=age`

**Logic Execution:**
1. `teenagers` $\rightarrow$ `age_group=teenager`
2. `female` $\rightarrow$ `gender=female`
3. `between 15 and 19` $\rightarrow$ `min_age=13`, `max_age=19`
4. Result: Sorted list of female teenagers within the specific age bracket.

## Setup & Technical Requirements
* **Go Version**: 1.25+ (utilizes `slices.Contains`)
* **Database**: SQLite3
* **Validation**: Custom error wrapping with `%w` for traceability.



