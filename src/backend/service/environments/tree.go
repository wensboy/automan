// 本文件负责树的递归读取与目录复制/删除(由 store 指令组合实现)。
package environments

import (
	"errors"
	"os"
	"path"
	"sort"
	"strings"

	domainenvironments "github.com/wensboy/automan/backend/domain/environments"
)

func (s *Service) treeLocked(rel string) ([]domainenvironments.Node, error) {
	names, err := s.list(rel)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []domainenvironments.Node{}, nil
		}
		return nil, err
	}

	nodes := make([]domainenvironments.Node, 0, len(names))
	for _, name := range names {
		childRel := path.Join(rel, name)
		info, statErr := s.stat(childRel)
		if statErr != nil {
			continue
		}
		if info.IsDir {
			children, childErr := s.treeLocked(childRel)
			if childErr != nil {
				return nil, childErr
			}
			nodes = append(nodes, domainenvironments.Node{Name: name, Path: childRel, Type: domainenvironments.NodeDir, Children: children})
			continue
		}
		if !strings.HasSuffix(strings.ToLower(name), ".json") {
			continue
		}
		nodes = append(nodes, domainenvironments.Node{Name: name, Path: childRel, Type: domainenvironments.NodeFile})
	}

	sort.SliceStable(nodes, func(i, j int) bool {
		if nodes[i].Type != nodes[j].Type {
			return nodes[i].Type == domainenvironments.NodeDir
		}
		return strings.ToLower(nodes[i].Name) < strings.ToLower(nodes[j].Name)
	})
	return nodes, nil
}

func (s *Service) listAllFiles() ([]string, error) {
	result := []string{}
	var walk func(rel string) error
	walk = func(rel string) error {
		names, err := s.list(rel)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		for _, name := range names {
			childRel := path.Join(rel, name)
			info, statErr := s.stat(childRel)
			if statErr != nil {
				continue
			}
			if info.IsDir {
				if err := walk(childRel); err != nil {
					return err
				}
				continue
			}
			if strings.HasSuffix(strings.ToLower(name), ".json") {
				result = append(result, childRel)
			}
		}
		return nil
	}
	if err := walk(""); err != nil {
		return nil, err
	}
	sort.Strings(result)
	return result, nil
}

func (s *Service) copyTree(from, to string) error {
	info, err := s.stat(from)
	if err != nil {
		return err
	}
	if !info.IsDir {
		payload, readErr := s.readFile(from)
		if readErr != nil {
			return readErr
		}
		return s.writeFile(to, payload)
	}
	if err := s.mkdir(to); err != nil {
		return err
	}
	names, err := s.list(from)
	if err != nil {
		return err
	}
	for _, name := range names {
		if err := s.copyTree(path.Join(from, name), path.Join(to, name)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) removeAll(rel string) error {
	info, err := s.stat(rel)
	if err != nil {
		return err
	}
	if !info.IsDir {
		return s.remove(rel)
	}
	names, err := s.list(rel)
	if err != nil {
		return err
	}
	for _, name := range names {
		if err := s.removeAll(path.Join(rel, name)); err != nil {
			return err
		}
	}
	return s.remove(rel)
}
