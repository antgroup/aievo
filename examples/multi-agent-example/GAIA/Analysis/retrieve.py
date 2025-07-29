import json
import os
import numpy as np
import torch
from sentence_transformers import SentenceTransformer


def create_query_embeddings():
    """
    This function processes JSON files containing questions and analyses,
    encoif __name__ == "__main__":

    # create_query_embeddings()
    # create_repo_embeddings()
    retrieve_and_rank()them using a pre-trained SentenceTransformer model, and saves
    the embeddings to disk.
    """
    # Create embedding directory if it doesn't exist
    if not os.path.exists("embedding"):
        os.makedirs("embedding")

    # Load the sentence transformer model
    model = SentenceTransformer("/home/liuguangyi/Qwen3-Embedding-8B", device="cuda:2")

    # List of files to process
    files_to_process = ["anal_level_1.json", "anal_level_2.json", "anal_level_3.json"]

    for filename in files_to_process:
        print(f"Processing {filename}...")
        
        # Load the JSON data
        with open(filename, 'r') as f:
            data = json.load(f)
        
        # Extract questions and analyses
        questions = [item['question'] for item in data]
        analyses = [item['analysis'] for item in data]
        
        # Encode the questions and analyses
        print("Encoding questions...")
        question_embeddings = model.encode(questions, prompt_name="query")
        
        print("Encoding analyses...")
        analysis_embeddings = model.encode(analyses)
        
        # Get the base name of the file
        base_name = os.path.splitext(filename)[0][5:]
        
        # Save the embeddings
        question_embedding_file = os.path.join("embedding", f"{base_name}_qs_emb.npy")
        analysis_embedding_file = os.path.join("embedding", f"{base_name}_as_emb.npy")
        
        print(f"Saving question embeddings to {question_embedding_file}...")
        np.save(question_embedding_file, question_embeddings)
        
        print(f"Saving analysis embeddings to {analysis_embedding_file}...")
        np.save(analysis_embedding_file, analysis_embeddings)
        
        print(f"Finished processing {filename}.")

    print("All files processed and embeddings saved.")


def create_repo_embeddings():
    """
    This function reads all JSON files from the SOP generation directory,
    extracts questions and analyses, encodes them, and saves the embeddings
    into two aggregated files.
    """
    # Create embedding directory if it doesn't exist
    embedding_dir = "embedding"
    if not os.path.exists(embedding_dir):
        os.makedirs(embedding_dir)

    # Load the sentence transformer model
    model = SentenceTransformer("/home/liuguangyi/Qwen3-Embedding-8B", device="cuda:2")

    # Path to the SOP generation directory
    sop_dir = "../SOP/gen_sop"
    
    # List of files to process
    try:
        files_to_process = [os.path.join(sop_dir, f) for f in os.listdir(sop_dir) if f.endswith('.json')]
    except FileNotFoundError:
        print(f"Error: Directory not found at {os.path.abspath(sop_dir)}")
        return

    all_questions = []
    all_analyses = []

    for filename in files_to_process:
        print(f"Processing {filename}...")
        
        # Load the JSON data
        with open(filename, 'r') as f:
            try:
                data = json.load(f)
            except json.JSONDecodeError:
                print(f"Skipping invalid JSON file: {filename}")
                continue
        
        # The data in SOP files is a single dictionary, not a list
        if isinstance(data, dict):
            question = data.get('question', '')
            if question.startswith("Question:"):
                question = question.replace("Question:", "", 1).strip()
            
            all_questions.append(question)
            all_analyses.append(data.get('analysis', ''))
        else:
            print(f"Skipping file with unexpected format: {filename}")

    if not all_questions:
        print("No questions found to process.")
        return

    # Encode all questions and analyses
    print("Encoding all questions...")
    question_embeddings = model.encode(all_questions, prompt_name="query")
    
    print("Encoding all analyses...")
    analysis_embeddings = model.encode(all_analyses)
    
    # Save the aggregated embeddings
    question_embedding_file = os.path.join(embedding_dir, "repo_qs_emb.npy")
    analysis_embedding_file = os.path.join(embedding_dir, "repo_as_emb.npy")
    
    print(f"Saving all question embeddings to {question_embedding_file}...")
    np.save(question_embedding_file, question_embeddings)
    
    print(f"Saving all analysis embeddings to {analysis_embedding_file}...")
    np.save(analysis_embedding_file, analysis_embeddings)

    print("All SOP files processed and aggregated embeddings saved.")

def retrieve_and_rank():
    """
    Performs retrieval based on similarity metrics between query and repo embeddings.
    For each query, it finds the top 3 most similar repo items based on three different strategies
    and saves the results to JSON files.
    """
    print("Starting retrieval and ranking process...")
    
    # Load the sentence transformer model for similarity calculation
    model = SentenceTransformer("/home/liuguangyi/Qwen3-Embedding-8B", device="cuda:2")
    
    embedding_dir = "embedding"
    sop_dir = "../SOP/gen_sop"

    # Load repository embeddings and IDs
    try:
        repo_qs_emb = np.load(os.path.join(embedding_dir, "repo_qs_emb.npy"))
        repo_as_emb = np.load(os.path.join(embedding_dir, "repo_as_emb.npy"))
        
        # The IDs are the filenames from the SOP directory
        repo_ids = sorted([f for f in os.listdir(sop_dir) if f.endswith('.json')])
        
        if len(repo_ids) != repo_qs_emb.shape[0]:
            print("Warning: Mismatch between number of repo IDs and embeddings. Check for changes in SOP directory.")

    except FileNotFoundError:
        print("Error: Repository embeddings not found. Please run 'create_repo_embeddings' first.")
        return

    # Process each level
    for level in [1, 2, 3]:
        print(f"\nProcessing Level {level}...")
        level_results = []
        
        try:
            # Load query embeddings for the current level
            query_qs_emb = np.load(os.path.join(embedding_dir, f"level_{level}_qs_emb.npy"))
            query_as_emb = np.load(os.path.join(embedding_dir, f"level_{level}_as_emb.npy"))
            
            # Load original query data to have a reference
            with open(f"anal_level_{level}.json", 'r') as f:
                queries_data = json.load(f)

        except FileNotFoundError:
            print(f"Error: Embeddings or data file for level {level} not found. Please run 'create_query_embeddings' first.")
            continue

        # Calculate similarities
        # Shape: (num_queries, num_repo_items)
        sim_qs = model.similarity(query_qs_emb, repo_qs_emb)
        sim_as = model.similarity(query_as_emb, repo_as_emb)
        
        # Ensure tensors are on the CPU for numpy operations
        if isinstance(sim_qs, torch.Tensor):
            sim_qs = sim_qs.cpu().numpy()
        if isinstance(sim_as, torch.Tensor):
            sim_as = sim_as.cpu().numpy()
        
        # Combined similarity
        sim_combined = 0.5 * sim_qs + 0.5 * sim_as

        # For each query in the level, find top 3 repo IDs for each method
        for i, query_data in enumerate(queries_data):
            
            # Method 1: Question similarity - Sort descending and take top 3
            top_3_qs_indices = np.argsort(-sim_qs[i])[:3]
            top_3_qs_ids = [repo_ids[j] for j in top_3_qs_indices]

            # Method 2: Analysis similarity - Sort descending and take top 3
            top_3_as_indices = np.argsort(-sim_as[i])[:3]
            top_3_as_ids = [repo_ids[j] for j in top_3_as_indices]

            # Method 3: Combined similarity - Sort descending and take top 3
            top_3_combined_indices = np.argsort(-sim_combined[i])[:3]
            top_3_combined_ids = [repo_ids[j] for j in top_3_combined_indices]

            level_results.append({
                "id": query_data["id"],
                "query_question": query_data["question"],
                "retrieval_results": {
                    "question_similarity": top_3_qs_ids,
                    "analysis_similarity": top_3_as_ids,
                    "weighted_similarity": top_3_combined_ids,
                }
            })

        # Save results for the level to a JSON file
        output_filename = f"retri_results_level_{level}.json"
        with open(output_filename, 'w') as f:
            json.dump(level_results, f, indent=4)
            
        print(f"Level {level} results saved to {output_filename}")

    print("\nAll levels processed. Retrieval and ranking complete.")

if __name__ == "__main__":

    # create_query_embeddings()
    # create_repo_embeddings()
    retrieve_and_rank()
