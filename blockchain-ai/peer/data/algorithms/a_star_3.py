def read_graph(file_path):
    graph = {}
    with open(file_path, 'r') as file:
        for line in file:
            node, edges = line.split(':')
            graph[node.strip()] = {edge.split(',')[0]: int(edge.split(',')[1]) for edge in edges.split()}
    return graph

def a_star(graph, start, goal):
    open_set = [(0, start)]
    came_from = {}
    g_score = {node: float('inf') for node in graph}
    g_score[start] = 0

    while open_set:
        open_set.sort()  # Manual priority queue
        _, current = open_set.pop(0)
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
                open_set.append((tentative_g_score, neighbor))
    return None

if __name__ == "__main__":
    graph = read_graph("graph_3.txt")
    print(a_star(graph, "1", "4"))