package metrics

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// PodMetric is one pod's aggregate resource usage across all containers.
type PodMetric struct {
	Name          string
	Namespace     string
	CPUMillicores int64
	MemoryBytes   int64
}

// NodeMetric is one node's resource usage sample.
type NodeMetric struct {
	Name          string
	CPUMillicores int64
	MemoryBytes   int64
}

func parsePodMetrics(items []unstructured.Unstructured) ([]PodMetric, error) {
	result := make([]PodMetric, 0, len(items))
	for _, item := range items {
		name := item.GetName()
		namespace := item.GetNamespace()
		containers, found, err := unstructured.NestedSlice(item.Object, "containers")
		if err != nil {
			return nil, fmt.Errorf("E_METRICS_UNAVAILABLE: parse pod %q containers: %w", name, err)
		}
		if !found || len(containers) == 0 {
			continue
		}
		cpu, memory, hasCPU, hasMemory, err := sumContainerUsage(containers)
		if err != nil {
			return nil, err
		}
		if !hasCPU || !hasMemory {
			continue
		}
		result = append(result, PodMetric{
			Name: name, Namespace: namespace,
			CPUMillicores: cpu.MilliValue(), MemoryBytes: memory.Value(),
		})
	}
	return result, nil
}

func parseNodeMetrics(items []unstructured.Unstructured) ([]NodeMetric, error) {
	result := make([]NodeMetric, 0, len(items))
	for _, item := range items {
		usage, found, err := unstructured.NestedStringMap(item.Object, "usage")
		if err != nil {
			return nil, fmt.Errorf("E_METRICS_UNAVAILABLE: parse node %q usage: %w", item.GetName(), err)
		}
		if !found {
			continue
		}
		cpu, hasCPU, err := parseQuantity(usage["cpu"])
		if err != nil {
			return nil, err
		}
		memory, hasMemory, err := parseQuantity(usage["memory"])
		if err != nil {
			return nil, err
		}
		if !hasCPU || !hasMemory {
			continue
		}
		result = append(result, NodeMetric{Name: item.GetName(), CPUMillicores: cpu.MilliValue(), MemoryBytes: memory.Value()})
	}
	return result, nil
}

func sumContainerUsage(containers []any) (resource.Quantity, resource.Quantity, bool, bool, error) {
	var cpu, memory resource.Quantity
	hasCPU, hasMemory := false, false
	for _, raw := range containers {
		container, ok := raw.(map[string]any)
		if !ok {
			return resource.Quantity{}, resource.Quantity{}, false, false, fmt.Errorf("E_METRICS_UNAVAILABLE: container metric is not an object")
		}
		usage, ok := container["usage"].(map[string]any)
		if !ok {
			continue
		}
		cpuValue, cpuFound, err := quantityFromAny(usage["cpu"])
		if err != nil {
			return resource.Quantity{}, resource.Quantity{}, false, false, err
		}
		memoryValue, memoryFound, err := quantityFromAny(usage["memory"])
		if err != nil {
			return resource.Quantity{}, resource.Quantity{}, false, false, err
		}
		if cpuFound {
			if !hasCPU {
				cpu = cpuValue
			} else {
				cpu.Add(cpuValue)
			}
			hasCPU = true
		}
		if memoryFound {
			if !hasMemory {
				memory = memoryValue
			} else {
				memory.Add(memoryValue)
			}
			hasMemory = true
		}
	}
	return cpu, memory, hasCPU, hasMemory, nil
}

func quantityFromAny(value any) (resource.Quantity, bool, error) {
	text, ok := value.(string)
	if !ok || text == "" {
		return resource.Quantity{}, false, nil
	}
	return parseQuantity(text)
}

func parseQuantity(text string) (resource.Quantity, bool, error) {
	if text == "" {
		return resource.Quantity{}, false, nil
	}
	quantity, err := resource.ParseQuantity(text)
	if err != nil {
		return resource.Quantity{}, false, fmt.Errorf("E_METRICS_UNAVAILABLE: invalid resource quantity %q: %w", text, err)
	}
	return quantity, true, nil
}
