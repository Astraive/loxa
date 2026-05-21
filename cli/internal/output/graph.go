package output

import "fmt"

func RenderASCIIGraph(graph map[string]any) string {
	nodes, _ := graph["nodes"].([]any)
	edges, _ := graph["edges"].([]any)

	if len(nodes) == 0 {
		return "  (empty graph)"
	}

	result := ""
	nodeMap := map[string]map[string]any{}
	for _, n := range nodes {
		if m, ok := n.(map[string]any); ok {
			id := fmt.Sprintf("%v", m["id"])
			nodeMap[id] = m
			label := fmt.Sprintf("%v", m["label"])
			if label == "" || label == "<nil>" {
				label = id
			}
			ntype := fmt.Sprintf("%v", m["type"])
			switch ntype {
			case "service":
				result += fmt.Sprintf("  [%s] %s\n", Info("SVC"), Bold(label))
			case "event":
				result += fmt.Sprintf("  [%s] %s\n", Dim("EVT"), label)
			default:
				result += fmt.Sprintf("  [%s] %s\n", Dim(ntype), label)
			}
		}
	}

	if len(edges) > 0 {
		result += "\n"
		for _, e := range edges {
			if m, ok := e.(map[string]any); ok {
				from := fmt.Sprintf("%v", m["from_node_id"])
				if from == "" || from == "<nil>" {
					from = fmt.Sprintf("%v", m["source"])
				}
				to := fmt.Sprintf("%v", m["to_node_id"])
				if to == "" || to == "<nil>" {
					to = fmt.Sprintf("%v", m["target"])
				}
				label := fmt.Sprintf("%v", m["label"])
				if label == "" || label == "<nil>" {
					label = fmt.Sprintf("%v", m["edge_type"])
				}
				fromLabel := from
				toLabel := to
				if n, ok := nodeMap[from]; ok {
					if l, ok := n["label"].(string); ok && l != "" {
						fromLabel = l
					}
				}
				if n, ok := nodeMap[to]; ok {
					if l, ok := n["label"].(string); ok && l != "" {
						toLabel = l
					}
				}
				result += fmt.Sprintf("  %s --%s--> %s\n", fromLabel, Dim(label), toLabel)
			}
		}
	}

	return result
}
