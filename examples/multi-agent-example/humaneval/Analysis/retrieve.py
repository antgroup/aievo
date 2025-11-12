import json
import os
import numpy as np
import torch
from sentence_transformers import SentenceTransformer, util
from FlagEmbedding import BGEM3FlagModel

# Define a global device variable
device = "cuda:7"


def create_query_embeddings(model_name='qwen'):
    """
    Create query embeddings from anal_valid.json, saving to embedding_<model>/valid_*_emb.npy.
    :param model_name: The model to use for encoding ('qwen' or 'bge').
    """
    # Create embedding directory if it doesn't exist
    embedding_dir = f"embedding_{model_name}"
    if not os.path.exists(embedding_dir):
        os.makedirs(embedding_dir)

    # Load the sentence transformer model
    if model_name == 'qwen':
        model = SentenceTransformer("/home/liuguangyi/Qwen3-Embedding-8B", device=device)
    elif model_name == 'bge':
        model = BGEM3FlagModel('BAAI/bge-m3', use_fp16=True, device=device)
    else:
        raise ValueError("Unsupported model_name. Choose 'qwen' or 'bge'.")

    # List of files to process (queries come from validation set)
    # files_to_process = ["anal_valid.json"]
    files_to_process = ["anal_pro.json"]

    for filename in files_to_process:
        print(f"Processing {filename} with {model_name} model...")
        
        # Load the JSON data
        with open(filename, 'r') as f:
            data = json.load(f)
        
        # Extract questions and analyses
        questions = [item['question'] for item in data]
        analyses = [item['analysis'] for item in data]
        
        # Encode the questions and analyses
        print("Encoding questions...")
        if model_name == 'qwen':
            question_embeddings = model.encode(questions, prompt_name="query")
            print("Encoding analyses...")
            analysis_embeddings = model.encode(analyses)
        else: # bge
            output_qs = model.encode(questions, return_dense=True, return_sparse=False, return_colbert_vecs=True)
            question_embeddings = output_qs['colbert_vecs']
            print("Encoding analyses...")
            output_as = model.encode(analyses, return_dense=True, return_sparse=False, return_colbert_vecs=True)
            analysis_embeddings = output_as['colbert_vecs']

        # Get the base name of the file (strip 'anal_' prefix)
        base_name = os.path.splitext(filename)[0][5:]

        # Save the embeddings
        question_embedding_file = os.path.join(embedding_dir, f"{base_name}_qs_emb.npy")
        analysis_embedding_file = os.path.join(embedding_dir, f"{base_name}_as_emb.npy")

        print(f"Saving question embeddings to {question_embedding_file}...")
        np.save(question_embedding_file, question_embeddings, allow_pickle=True)

        print(f"Saving analysis embeddings to {analysis_embedding_file}...")
        np.save(analysis_embedding_file, analysis_embeddings, allow_pickle=True)

        print(f"Finished processing {filename}.")

    print("All files processed and embeddings saved.")


def create_repo_embeddings(model_name='qwen'):
    """
    Create repository embeddings from anal_train.json, saving to embedding_<model>/repo_*_emb.npy.
    :param model_name: The model to use for encoding ('qwen' or 'bge').
    """
    # Create embedding directory if it doesn't exist
    embedding_dir = f"embedding_{model_name}"
    if not os.path.exists(embedding_dir):
        os.makedirs(embedding_dir)

    # Load the sentence transformer model
    if model_name == 'qwen':
        model = SentenceTransformer("/home/liuguangyi/Qwen3-Embedding-8B", device=device)
    elif model_name == 'bge':
        model = BGEM3FlagModel('BAAI/bge-m3', use_fp16=True, device=device)
    else:
        raise ValueError("Unsupported model_name. Choose 'qwen' or 'bge'.")


    # Repository is the training set
    files_to_process = ["anal_train.json"]
    all_questions = []
    all_analyses = []

    for filename in files_to_process:
        print(f"Processing {filename} with {model_name} model...")
        
        # Load the JSON data
        with open(filename, 'r') as f:
            data = json.load(f)
        
        # Extract questions and analyses
        questions = [item['question'] for item in data]
        analyses = [item['analysis'] for item in data]

        all_questions.extend(questions)
        all_analyses.extend(analyses)

    # Encode all questions and analyses
    print(f"Encoding all questions with {model_name} model...")
    if model_name == 'qwen':
        question_embeddings = model.encode(all_questions, prompt_name="query")
        print("Encoding all analyses...")
        analysis_embeddings = model.encode(all_analyses)
    else: # bge
        output_qs = model.encode(all_questions, return_dense=True, return_sparse=False, return_colbert_vecs=True)
        question_embeddings = output_qs['colbert_vecs']
        print("Encoding all analyses...")
        output_as = model.encode(all_analyses, return_dense=True, return_sparse=False, return_colbert_vecs=True)
        analysis_embeddings = output_as['colbert_vecs']

    # Save the aggregated embeddings
    question_embedding_file = os.path.join(embedding_dir, "repo_qs_emb.npy")
    analysis_embedding_file = os.path.join(embedding_dir, "repo_as_emb.npy")
    
    print(f"Saving all question embeddings to {question_embedding_file}...")
    np.save(question_embedding_file, question_embeddings, allow_pickle=True)
    
    print(f"Saving all analysis embeddings to {analysis_embedding_file}...")
    np.save(analysis_embedding_file, analysis_embeddings, allow_pickle=True)

    print("All SOP files processed and aggregated embeddings saved.")

def retrieve_and_rank(model_name='qwen'):
    """
    Perform retrieval using repo embeddings from anal_train.json and query embeddings from anal_valid.json.
    For each valid query, find the top-3 most similar training items by three strategies and save a single JSON result.
    :param model_name: The model used for embeddings ('qwen' or 'bge').
    """
    print(f"Starting retrieval and ranking process for {model_name} model...")
    
    embedding_dir = f"embedding_{model_name}"

    # Load repository embeddings and IDs from anal_train.json
    try:
        repo_qs_emb = np.load(os.path.join(embedding_dir, "repo_qs_emb.npy"), allow_pickle=True)
        repo_as_emb = np.load(os.path.join(embedding_dir, "repo_as_emb.npy"), allow_pickle=True)
        with open("anal_train.json", 'r') as f:
            repo_data = json.load(f)
        repo_ids = [ item.get('id') for idx, item in enumerate(repo_data) ]
        if len(repo_ids) != repo_qs_emb.shape[0]:
            print("Warning: Mismatch between number of train items and repo embeddings. Embeddings and data must align.")

    except FileNotFoundError:
        print(f"Error: Repository embeddings not found. Please run 'create_repo_embeddings(model_name=\"{model_name}\")' first.")
        return

    # Load model for bge similarity calculation
    if model_name == 'bge':
        model = BGEM3FlagModel('BAAI/bge-m3', use_fp16=True, device=device)

    # mode = "valid"
    mode = "pro"
    # Load query embeddings and query data from anal_valid.json
    try:
        query_qs_emb = np.load(os.path.join(embedding_dir, f"{mode}_qs_emb.npy"), allow_pickle=True)
        query_as_emb = np.load(os.path.join(embedding_dir, f"{mode}_as_emb.npy"), allow_pickle=True)
        with open(f"anal_{mode}.json", 'r') as f:
            queries_data = json.load(f)
    except FileNotFoundError:
        print(f"Error: Embeddings or data file for valid set not found. Please run 'create_query_embeddings(model_name=\"{model_name}\")' first.")
        return

    # Calculate similarities
    if model_name == 'qwen':
        # Shape: (num_queries, num_repo_items)
        sim_qs = util.cos_sim(query_qs_emb, repo_qs_emb)
        sim_as = util.cos_sim(query_as_emb, repo_as_emb)

        # Ensure tensors are on the CPU for numpy operations
        if isinstance(sim_qs, torch.Tensor):
            sim_qs = sim_qs.cpu().numpy()
        if isinstance(sim_as, torch.Tensor):
            sim_as = sim_as.cpu().numpy()
    else:  # bge
        num_queries = len(query_qs_emb)
        num_repos = len(repo_qs_emb)
        sim_qs = np.zeros((num_queries, num_repos))
        sim_as = np.zeros((num_queries, num_repos))
        print("Calculating BGE similarities...")
        for i in range(num_queries):
            for j in range(num_repos):
                sim_qs[i, j] = model.colbert_score(query_qs_emb[i], repo_qs_emb[j])
                sim_as[i, j] = model.colbert_score(query_as_emb[i], repo_as_emb[j])

    # Combined similarity
    sim_combined = 0.5 * sim_qs + 0.5 * sim_as

    # For each valid query, find top-3 training items for each method
    results = []
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

        results.append({
            "id": query_data.get("id", i),
            "query_question": query_data.get("question", ""),
            "retrieval_results": {
                "question_similarity": top_3_qs_ids,
                "analysis_similarity": top_3_as_ids,
                "weighted_similarity": top_3_combined_ids,
            }
        })

    # Save results for the valid set to a JSON file
    output_filename = f"retri_results_{mode}_{model_name}.json"
    with open(output_filename, 'w') as f:
        json.dump(results, f, indent=4)

    print(f"Valid set results saved to {output_filename}")
    print("\nRetrieval and ranking complete.")

if __name__ == "__main__":
    
    # Set the model to use: 'qwen' or 'bge'
    model_to_use = 'qwen' 

    create_query_embeddings(model_name=model_to_use)
    # create_repo_embeddings(model_name=model_to_use)
    retrieve_and_rank(model_name=model_to_use)
