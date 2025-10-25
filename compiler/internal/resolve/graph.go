package resolve

// Graph is a simple directed graph keyed by module path strings.
type Graph struct {
	edges map[string][]string
}

func NewGraph() *Graph {
	return &Graph{edges: map[string][]string{}}
}

func (g *Graph) AddEdge(from, to string) {
	if g.edges[from] == nil {
		g.edges[from] = []string{}
	}
	g.edges[from] = append(g.edges[from], to)
}

// Cycles returns all back-edge cycles detected with a DFS.
// For Phase-1, it’s fine to return the first cycle discovered; but we surface all simple cycles we hit.
func (g *Graph) Cycles() [][]string {
	var (
		color = map[string]int{} // 0=white,1=gray,2=black
		stack []string
		out   [][]string
	)

	var dfs func(u string)
	dfs = func(u string) {
		color[u] = 1
		stack = append(stack, u)
		for _, v := range g.edges[u] {
			if color[v] == 0 {
				dfs(v)
			} else if color[v] == 1 {
				// Found a back-edge; extract cycle v..u
				cycle := []string{v}
				for i := len(stack) - 1; i >= 0; i-- {
					cycle = append(cycle, stack[i])
					if stack[i] == v {
						break
					}
				}
				out = append(out, reverse(cycle))
			}
		}
		stack = stack[:len(stack)-1]
		color[u] = 2
	}

	for u := range g.edges {
		if color[u] == 0 {
			dfs(u)
		}
	}
	return out
}

func reverse[T any](s []T) []T {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}
