from flask import Flask, request, jsonify

app = Flask(__name__)

# In-memory "database" for queues
queues = {}
next_queue_id = 1

@app.route('/queue/add', methods=['POST'])
def add_queue():
    global next_queue_id

    # Get data from request
    data = request.json
    title = data.get("title")
    members = data.get("members", [])

    # Create the queue entry
    queue_id = next_queue_id
    next_queue_id += 1
    queues[queue_id] = {
        "title": title,
        "members": members,
        "status": "in-progress"
    }

    return jsonify({"message": "Queue added successfully", "queue_id": queue_id}), 201


@app.route('/queue/list', methods=['GET'])
def list_queues():
    return jsonify(queues)


@app.route('/queue/remove/<int:queue_id>', methods=['DELETE'])
def remove_queue(queue_id):
    if queue_id in queues:
        del queues[queue_id]
        return jsonify({"message": f"Queue {queue_id} removed successfully"}), 200
    return jsonify({"error": "Queue not found"}), 404


@app.route('/queue/approve/<int:queue_id>', methods=['POST'])
def approve_queue(queue_id):
    if queue_id not in queues:
        return jsonify({"error": "Queue not found"}), 404
    
    # Approve tagged members from the queue
    members = queues[queue_id]["members"]
    if members:
        members.pop(0)  # Remove the first member in the list (approve them)
    
    if not members:
        queues[queue_id]["status"] = "completed"

    return jsonify({"message": "Queue approved", "queue_id": queue_id, "status": queues[queue_id]["status"]})


@app.route('/queue/report', methods=['GET'])
def report_queues():
    completed_queues = [q for q in queues.values() if q["status"] == "completed"]
    return jsonify({"completed_queues": completed_queues})


if __name__ == "__main__":
    app.run(debug=True)
