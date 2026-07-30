#!/bin/bash
cd /c/viral/desilang/desi || exit 1
OUT=/tmp/dblocks3
rm -rf "$OUT"; mkdir -p "$OUT/dd"
find book/docs -name '*.md' | while read -r md; do
  awk -v FILE="$md" -v OUT="$OUT" '
    /^[ \t]*```desi[ \t]*$/ { inb=1; start=NR; buf=""; next }
    /^[ \t]*```[ \t]*$/ {
      if (inb) { key=FILE"#"start; gsub(/[\/.]/,"_",key)
                 f=OUT"/"key".desi"; printf "%s", buf > f; close(f)
                 print FILE"\t"start"\t"f >> OUT"/index.tsv" }
      inb=0; next }
    inb { buf = buf $0 "\n" }
  ' "$md"
done
: > "$OUT/frags_dd.tsv"
while IFS=$'\t' read -r md line f; do
  grep -q "def main" "$f" && continue
  dst="$OUT/dd/$(basename "$f")"
  awk '{lines[NR]=$0; if($0~/^[ \t]*$/) next; match($0,/^[ \t]*/); n=RLENGTH; if(min==""||n<min) min=n}
       END{for(i=1;i<=NR;i++) print substr(lines[i],min+1)}' "$f" > "$dst"
  printf '%s\t%s\t%s\n' "$md" "$line" "$dst" >> "$OUT/frags_dd.tsv"
done < "$OUT/index.tsv"
echo "blocks=$(ls $OUT/*.desi | wc -l) fragments=$(wc -l < $OUT/frags_dd.tsv)"
