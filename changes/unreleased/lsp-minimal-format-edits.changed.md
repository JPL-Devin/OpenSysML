- **Formatting in the editor changes only the lines that need it, and can format a selection.**
  `textDocument/formatting` used to answer with one edit replacing the whole document, so a
  reformat collapsed the undo history into a single step and moved the cursor, selection and
  folds. The server now answers with one small edit per changed region — an indentation fix on
  the one line that needs it, a deletion of the one surplus blank line — and nothing for a
  document that is already formatted, so the editor keeps its cursor, selection, folds and
  undo history across a format. `textDocument/rangeFormatting` is now implemented: a selection
  (widened to whole lines) gets only the edits on those lines, indented from the whole file's
  structure so the result matches its surroundings.
