import heapq
import sys
def read_graph(file_path):
    graph = {}
    with open(file_path, 'r') as file:
        for line in file:
            node, edges = line.split(':')
            graph[node.strip()] = {edge.split(',')[0]: int(edge.split(',')[1]) for edge in edges.split()}
    return graph

def a_star(graph, start, goal):
    open_set = []
    heapq.heappush(open_set, (0, start))
    came_from = {}
    g_score = {node: float('inf') for node in graph}
    g_score[start] = 0
    f_score = {node: float('inf') for node in graph}
    f_score[start] = 0

    while open_set:
        _, current = heapq.heappop(open_set)
        if current == goal:
            path = []
            while current in came_from:
                path.append(current)
                current = came_from[current]
            path.append(start)
            return path[::-1]

        for neighbor, cost in graph[current].items():
            tentative_g_score = g_score[current] + cost
            if tentative_g_score < g_score[neighbor]:
                came_from[neighbor] = current
                g_score[neighbor] = tentative_g_score
                f_score[neighbor] = g_score[neighbor]
                heapq.heappush(open_set, (f_score[neighbor], neighbor))
    return None

if __name__ == "__main__":
    graph_file = sys.argv[1]
    graph = read_graph(graph_file)
    print(a_star(graph, "A", "D"))