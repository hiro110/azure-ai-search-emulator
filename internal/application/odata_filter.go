package application

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// ParseODataFilter parses an OData $filter expression and returns a SQL WHERE
// fragment (no WHERE keyword) using json_extract() for SQLite document blobs,
// plus the corresponding bind arguments.
func ParseODataFilter(filter string) (string, []interface{}, error) {
	p := &filterParser{input: strings.TrimSpace(filter)}
	sql, args, err := p.parseOrExpr()
	if err != nil {
		return "", nil, err
	}
	p.skipSpaces()
	if p.pos < len(p.input) {
		return "", nil, fmt.Errorf("unexpected input at position %d: %q", p.pos, p.input[p.pos:])
	}
	return sql, args, nil
}

// ParseODataOrderBy parses an OData $orderby expression and returns an SQL
// ORDER BY fragment (no ORDER BY keyword).
func ParseODataOrderBy(orderby string) (string, error) {
	parts := strings.Split(orderby, ",")
	sqlParts := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		fields := strings.Fields(part)
		fieldSQL := jsonExtract(fields[0])
		dir := "ASC"
		if len(fields) >= 2 {
			switch strings.ToLower(fields[1]) {
			case "desc":
				dir = "DESC"
			case "asc":
				dir = "ASC"
			default:
				return "", fmt.Errorf("invalid sort direction: %q", fields[1])
			}
		}
		sqlParts = append(sqlParts, fieldSQL+" "+dir)
	}
	return strings.Join(sqlParts, ", "), nil
}

func jsonExtract(field string) string {
	return fmt.Sprintf("json_extract(content, '$.%s')", field)
}

// filterParser is a recursive-descent parser for OData $filter expressions.
type filterParser struct {
	input string
	pos   int
}

func (p *filterParser) skipSpaces() {
	for p.pos < len(p.input) && unicode.IsSpace(rune(p.input[p.pos])) {
		p.pos++
	}
}

// parseOrExpr handles: expr ('or' expr)*
func (p *filterParser) parseOrExpr() (string, []interface{}, error) {
	left, args, err := p.parseAndExpr()
	if err != nil {
		return "", nil, err
	}
	for {
		p.skipSpaces()
		if !p.peekKeyword("or") {
			break
		}
		p.consumeKeyword("or")
		right, rightArgs, err := p.parseAndExpr()
		if err != nil {
			return "", nil, err
		}
		left = "(" + left + " OR " + right + ")"
		args = append(args, rightArgs...)
	}
	return left, args, nil
}

// parseAndExpr handles: expr ('and' expr)*
func (p *filterParser) parseAndExpr() (string, []interface{}, error) {
	left, args, err := p.parseNotExpr()
	if err != nil {
		return "", nil, err
	}
	for {
		p.skipSpaces()
		if !p.peekKeyword("and") {
			break
		}
		p.consumeKeyword("and")
		right, rightArgs, err := p.parseNotExpr()
		if err != nil {
			return "", nil, err
		}
		left = "(" + left + " AND " + right + ")"
		args = append(args, rightArgs...)
	}
	return left, args, nil
}

// parseNotExpr handles: ['not'] primary
func (p *filterParser) parseNotExpr() (string, []interface{}, error) {
	p.skipSpaces()
	if p.peekKeyword("not") {
		p.consumeKeyword("not")
		inner, args, err := p.parsePrimary()
		if err != nil {
			return "", nil, err
		}
		return "NOT (" + inner + ")", args, nil
	}
	return p.parsePrimary()
}

func (p *filterParser) parsePrimary() (string, []interface{}, error) {
	p.skipSpaces()
	if p.pos >= len(p.input) {
		return "", nil, fmt.Errorf("unexpected end of filter expression")
	}
	if p.input[p.pos] == '(' {
		p.pos++
		sql, args, err := p.parseOrExpr()
		if err != nil {
			return "", nil, err
		}
		p.skipSpaces()
		if p.pos >= len(p.input) || p.input[p.pos] != ')' {
			return "", nil, fmt.Errorf("expected ')' at position %d", p.pos)
		}
		p.pos++
		return sql, args, nil
	}
	if p.peekFunc("search.in") {
		return p.parseSearchIn()
	}
	if p.peekFunc("startswith") {
		return p.parseStartsWith()
	}
	return p.parseComparison()
}

func (p *filterParser) peekKeyword(kw string) bool {
	p.skipSpaces()
	end := p.pos + len(kw)
	if end > len(p.input) {
		return false
	}
	if !strings.EqualFold(p.input[p.pos:end], kw) {
		return false
	}
	// Must be followed by whitespace or end-of-input (word boundary).
	if end < len(p.input) && isIdentRune(rune(p.input[end])) {
		return false
	}
	return true
}

func (p *filterParser) consumeKeyword(kw string) {
	p.skipSpaces()
	p.pos += len(kw)
}

func (p *filterParser) peekFunc(name string) bool {
	p.skipSpaces()
	remaining := p.input[p.pos:]
	return len(remaining) >= len(name)+1 &&
		strings.EqualFold(remaining[:len(name)], name) &&
		remaining[len(name)] == '('
}

func isIdentRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.'
}

func (p *filterParser) parseIdentifier() (string, error) {
	p.skipSpaces()
	start := p.pos
	for p.pos < len(p.input) && isIdentRune(rune(p.input[p.pos])) {
		p.pos++
	}
	if p.pos == start {
		return "", fmt.Errorf("expected identifier at position %d", p.pos)
	}
	return p.input[start:p.pos], nil
}

func (p *filterParser) parseStringLiteral() (string, error) {
	p.skipSpaces()
	if p.pos >= len(p.input) || p.input[p.pos] != '\'' {
		return "", fmt.Errorf("expected string literal (single-quoted) at position %d", p.pos)
	}
	p.pos++ // consume opening '
	var sb strings.Builder
	for p.pos < len(p.input) {
		if p.input[p.pos] == '\'' {
			if p.pos+1 < len(p.input) && p.input[p.pos+1] == '\'' {
				sb.WriteByte('\'')
				p.pos += 2
			} else {
				p.pos++ // consume closing '
				return sb.String(), nil
			}
		} else {
			sb.WriteByte(p.input[p.pos])
			p.pos++
		}
	}
	return "", fmt.Errorf("unterminated string literal")
}

func (p *filterParser) parseComparison() (string, []interface{}, error) {
	field, err := p.parseIdentifier()
	if err != nil {
		return "", nil, err
	}
	p.skipSpaces()

	// Read the operator token (non-space sequence).
	opStart := p.pos
	for p.pos < len(p.input) && !unicode.IsSpace(rune(p.input[p.pos])) {
		p.pos++
	}
	op := strings.ToLower(p.input[opStart:p.pos])
	sqlOp, ok := map[string]string{
		"eq": "=", "ne": "!=", "gt": ">", "ge": ">=", "lt": "<", "le": "<=",
	}[op]
	if !ok {
		return "", nil, fmt.Errorf("unknown comparison operator: %q", op)
	}

	p.skipSpaces()
	return p.buildComparison(field, sqlOp)
}

func (p *filterParser) buildComparison(field, sqlOp string) (string, []interface{}, error) {
	if p.pos >= len(p.input) {
		return "", nil, fmt.Errorf("expected value at position %d", p.pos)
	}
	fieldExpr := jsonExtract(field)
	rest := strings.ToLower(p.input[p.pos:])

	// null
	if strings.HasPrefix(rest, "null") && !isIdentRune(rune(p.peek(4))) {
		p.pos += 4
		if sqlOp == "=" {
			return fieldExpr + " IS NULL", nil, nil
		}
		return fieldExpr + " IS NOT NULL", nil, nil
	}
	// true
	if strings.HasPrefix(rest, "true") && !isIdentRune(rune(p.peek(4))) {
		p.pos += 4
		return fieldExpr + " " + sqlOp + " ?", []interface{}{true}, nil
	}
	// false
	if strings.HasPrefix(rest, "false") && !isIdentRune(rune(p.peek(5))) {
		p.pos += 5
		return fieldExpr + " " + sqlOp + " ?", []interface{}{false}, nil
	}
	// string literal
	if p.input[p.pos] == '\'' {
		s, err := p.parseStringLiteral()
		if err != nil {
			return "", nil, err
		}
		return fieldExpr + " " + sqlOp + " ?", []interface{}{s}, nil
	}
	// number
	return p.parseNumber(fieldExpr, sqlOp)
}

// peek returns the rune at pos+offset, or 0 if out of range.
func (p *filterParser) peek(offset int) byte {
	i := p.pos + offset
	if i >= len(p.input) {
		return 0
	}
	return p.input[i]
}

func (p *filterParser) parseNumber(fieldExpr, sqlOp string) (string, []interface{}, error) {
	start := p.pos
	if p.pos < len(p.input) && p.input[p.pos] == '-' {
		p.pos++
	}
	for p.pos < len(p.input) && (unicode.IsDigit(rune(p.input[p.pos])) || p.input[p.pos] == '.') {
		p.pos++
	}
	numStr := p.input[start:p.pos]
	if numStr == "" || numStr == "-" {
		return "", nil, fmt.Errorf("expected numeric value at position %d", start)
	}
	if strings.Contains(numStr, ".") {
		f, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return "", nil, fmt.Errorf("invalid number %q: %w", numStr, err)
		}
		return fieldExpr + " " + sqlOp + " ?", []interface{}{f}, nil
	}
	n, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return "", nil, fmt.Errorf("invalid number %q: %w", numStr, err)
	}
	return fieldExpr + " " + sqlOp + " ?", []interface{}{n}, nil
}

// parseSearchIn handles: search.in(field, 'val1,val2' [, 'delimiter'])
func (p *filterParser) parseSearchIn() (string, []interface{}, error) {
	p.skipSpaces()
	p.pos += len("search.in(")

	field, err := p.parseIdentifier()
	if err != nil {
		return "", nil, fmt.Errorf("search.in: %w", err)
	}
	if err := p.expectComma("search.in"); err != nil {
		return "", nil, err
	}

	valList, err := p.parseStringLiteral()
	if err != nil {
		return "", nil, fmt.Errorf("search.in values: %w", err)
	}

	delimiter := ","
	p.skipSpaces()
	if p.pos < len(p.input) && p.input[p.pos] == ',' {
		p.pos++
		d, err := p.parseStringLiteral()
		if err != nil {
			return "", nil, fmt.Errorf("search.in delimiter: %w", err)
		}
		if d != "" {
			delimiter = d
		}
	}

	if err := p.expectCloseParen("search.in"); err != nil {
		return "", nil, err
	}

	values := strings.Split(valList, delimiter)
	fieldExpr := jsonExtract(field)
	placeholders := make([]string, len(values))
	args := make([]interface{}, len(values))
	for i, v := range values {
		placeholders[i] = "?"
		args[i] = strings.TrimSpace(v)
	}
	return fieldExpr + " IN (" + strings.Join(placeholders, ",") + ")", args, nil
}

// parseStartsWith handles: startswith(field, 'prefix')
func (p *filterParser) parseStartsWith() (string, []interface{}, error) {
	p.skipSpaces()
	p.pos += len("startswith(")

	field, err := p.parseIdentifier()
	if err != nil {
		return "", nil, fmt.Errorf("startswith: %w", err)
	}
	if err := p.expectComma("startswith"); err != nil {
		return "", nil, err
	}

	prefix, err := p.parseStringLiteral()
	if err != nil {
		return "", nil, fmt.Errorf("startswith: %w", err)
	}

	if err := p.expectCloseParen("startswith"); err != nil {
		return "", nil, err
	}

	fieldExpr := jsonExtract(field)
	// Escape LIKE metacharacters in prefix.
	escaped := strings.ReplaceAll(prefix, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, "%", `\%`)
	escaped = strings.ReplaceAll(escaped, "_", `\_`)
	return fieldExpr + ` LIKE ? ESCAPE '\'`, []interface{}{escaped + "%"}, nil
}

func (p *filterParser) expectComma(ctx string) error {
	p.skipSpaces()
	if p.pos >= len(p.input) || p.input[p.pos] != ',' {
		return fmt.Errorf("%s: expected ','", ctx)
	}
	p.pos++
	return nil
}

func (p *filterParser) expectCloseParen(ctx string) error {
	p.skipSpaces()
	if p.pos >= len(p.input) || p.input[p.pos] != ')' {
		return fmt.Errorf("%s: expected ')'", ctx)
	}
	p.pos++
	return nil
}
