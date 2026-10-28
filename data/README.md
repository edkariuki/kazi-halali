# Registry data

`sample/` holds **fictional** agencies so the app runs end to end without real data.
None of these names refer to real companies.

For the real snapshot, put files in `data/real/` (git-ignored) with the same columns,
point `AGENCIES_CSV` / `FLAGGED_CSV` at them, and run `make ingest`.

## agencies.csv

| column       | meaning                                                         |
|--------------|-----------------------------------------------------------------|
| `name`       | agency name exactly as NEA lists it                             |
| `nea_reg_no` | NEA registration / licence number, if shown                     |
| `status`     | `valid`, `under_verification`, `expired` or `unknown`           |
| `source_url` | where the row came from (e.g. the NEAIMS agency list page)      |

Source: NEAIMS "List of Private Employment Agencies". Agencies shown in black are
valid; those shown in red are still undergoing verification. Record the date you
took the snapshot — licence status changes.

## flagged.csv

| column         | meaning                                                   |
|----------------|-----------------------------------------------------------|
| `name`         | agency or company name as written in the official notice  |
| `reason`       | one line, paraphrasing the official notice                |
| `source_title` | title of the notice / report                              |
| `source_url`   | link to it                                                |
| `published_at` | YYYY-MM-DD                                                |

**Only add names that appear in an official government notice or report**, and
always keep the source link. Kazi Halali shows that link to users with every
finding, so a claim is never made without its source.
