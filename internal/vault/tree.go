package vault

import (
	"path"
	"sort"
	"strings"
)

func BuildTree(documents []DocumentMetadata) []TreeNode {
	root := make([]TreeNode, 0)

	for _, document := range documents {
		parts := splitDocumentPath(document.Path)
		if len(parts) == 0 {
			continue
		}
		insertNode(&root, parts, nil, document)
	}

	sortTree(root)
	return root
}

func splitDocumentPath(value string) []string {
	cleaned := strings.Trim(path.Clean(strings.ReplaceAll(value, "\\", "/")), "/")
	if cleaned == "." || cleaned == "" {
		return nil
	}
	return strings.Split(cleaned, "/")
}

func insertNode(nodes *[]TreeNode, parts []string, parentParts []string, document DocumentMetadata) {
	if len(parts) == 1 {
		*nodes = append(*nodes, TreeNode{
			ID:           document.ID,
			Title:        document.Title,
			Path:         strings.Join(append(parentParts, parts[0]), "/"),
			Kind:         "document",
			DocumentType: document.DocumentType,
			MimeType:     document.MimeType,
			Size:         document.Size,
		})
		return
	}

	folderTitle := parts[0]
	// 显式复制而非 append：append(parentParts, x) 可能复用 parentParts 的底层数组，
	// 同一父路径下多次插入时存在切片别名风险；复制后 folderParts 独立可控。
	folderParts := make([]string, len(parentParts)+1)
	copy(folderParts, parentParts)
	folderParts[len(parentParts)] = folderTitle
	folderPath := strings.Join(folderParts, "/")
	for i := range *nodes {
		if (*nodes)[i].Kind == "folder" && (*nodes)[i].Title == folderTitle {
			insertNode(&(*nodes)[i].Children, parts[1:], folderParts, document)
			return
		}
	}

	*nodes = append(*nodes, TreeNode{
		Title:    folderTitle,
		Path:     folderPath,
		Kind:     "folder",
		Children: []TreeNode{},
	})
	insertNode(&(*nodes)[len(*nodes)-1].Children, parts[1:], folderParts, document)
}

func sortTree(nodes []TreeNode) {
	sort.SliceStable(nodes, func(i, j int) bool {
		if nodes[i].Kind != nodes[j].Kind {
			return nodes[i].Kind == "folder"
		}
		return strings.ToLower(nodes[i].Title) < strings.ToLower(nodes[j].Title)
	})
	for i := range nodes {
		sortTree(nodes[i].Children)
	}
}
