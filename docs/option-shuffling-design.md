# Option Shuffling Design Note

This note captures a small product and technical design for shuffled answer options during exam sessions.

## Goal

Reduce the value of memorizing option positions while keeping question authoring, review, and result playback predictable.

## Proposed Rule

Keep the canonical option order in the question bank, but optionally shuffle the display order when an exam session is created.

This means:

- the question bank always stores the original author-defined order
- the admin UI always shows the original order
- an exam session may present a shuffled order
- the result page should replay the same order the user saw during that session

## Why This Approach

This avoids mixing two concerns:

- content management
- per-session presentation

The question bank should remain stable for import, editing, review, and support. Randomization should be a session behavior, not a data mutation.

## Product Rules

### Default Behavior

- single-choice and multi-choice questions may support option shuffling
- shuffling should be enabled or disabled by subject-level configuration
- a question may explicitly opt out of shuffling when its wording depends on option position

### Position-Sensitive Questions

Some questions should never be shuffled, for example:

- questions with options such as "all of the above" or "none of the above"
- questions whose stem references option position
- questions imported from legacy banks that depend on a fixed order

For these cases, the question should override the subject default and disable shuffling.

### Review And Results

- the result page should show options in the same order used during the actual exam session
- wrong-answer review should also use the session order
- the admin editor should continue to use canonical order

## Schema Impact

### Subject-Level Setting

Add a subject-level flag:

- `subjects.shuffle_options_default`

Suggested type:

- SQLite integer boolean with default `0`

Meaning:

- `0`: keep canonical option order during exams unless a question explicitly enables shuffling
- `1`: shuffle options during exams unless a question explicitly disables shuffling

### Question-Level Override

Add a nullable per-question override:

- `questions.allow_option_shuffle`

Suggested type:

- SQLite integer nullable boolean

Meaning:

- `NULL`: follow `subjects.shuffle_options_default`
- `0`: never shuffle this question's options
- `1`: always allow shuffling for this question

### Session-Level Persistence

The exam session needs to preserve the option order actually shown to the user.

Recommended addition:

- `exam_question_options.display_order`

This table should store one row per displayed option for one exam question instance.

Recommended columns:

- `id`
- `exam_question_id`
- `question_option_id`
- `display_order`

Why this is needed:

- result pages can replay the exact option order
- support and debugging can reconstruct the session
- future analytics can compare behavior across shuffled and non-shuffled sessions

## Runtime Behavior

### Exam Creation

When the system creates `exam_questions` for a new exam:

1. load canonical options in their stored order
2. decide whether shuffling applies for that question
3. if shuffling applies, randomize the option list
4. persist the final display order for that exam question

### Question Rendering

When rendering a question during the exam:

- read option order from the session-specific persisted order
- do not recompute shuffling on each request

This ensures a stable experience across refreshes and back/forward navigation.

### Result Rendering

When rendering results:

- use the persisted session order
- mark selected options and correct options against that same order

## Import And Admin Impact

### Importer

The Markdown import format does not need option-order randomization logic inside the option list itself.

Possible future extensions:

- subject manifest field for default option shuffling
- question-level metadata field for disabling or forcing shuffling

This can be added later without changing the canonical meaning of the option list.

### Admin UI

The admin editor should:

- keep showing canonical option order
- optionally expose a per-question "Allow option shuffling" field later

This should not be mixed with manual option reordering.

## Testing Impact

Minimum tests for a future implementation:

### Domain Behavior

- subject default off, question override null -> no shuffling
- subject default on, question override null -> shuffling allowed
- subject default on, question override off -> no shuffling
- subject default off, question override on -> shuffling allowed

### Session Stability

- one exam question keeps the same option order across repeated GET requests
- reloading the page does not reshuffle
- result page reuses the same persisted order

### Correctness

- correct answer matching still works after shuffling
- multi-choice answer submission still maps selected options correctly
- wrong-answer review highlights the correct options in the displayed order

### Safety

- position-sensitive questions marked as non-shufflable remain fixed
- admin editing does not mutate canonical option order during shuffle-related changes

## Recommended Rollout

Roll this out in two phases.

### Phase 1

- add schema fields
- persist per-session displayed option order
- keep subject default disabled

This creates the technical foundation without changing current user behavior.

### Phase 2

- add admin controls for subject default and question override
- enable shuffling for selected demo or practice subjects
- observe whether review and analytics behave as expected

## Recommendation

Implement shuffling as a session-level presentation feature, not as a question-bank rewrite.

That preserves authoring clarity while still making repeated practice less dependent on memorizing option positions.
