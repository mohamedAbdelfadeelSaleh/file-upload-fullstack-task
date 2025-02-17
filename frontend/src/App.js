import './App.css';
import { useRef, useState, useEffect } from 'react';
import axios from 'axios';

function App() {
  const [files, setFiles] = useState([]);
  const [uploadedFiles, setUploadedFiles] = useState([]);
  const [showProgress, setShowProgress] = useState(false);
  const [progress, setProgress] = useState({});
  const [totalProgress, setTotalProgress] = useState({
    upload: 0,
    processing: 0
  });
  const [students, setStudents] = useState([]);
  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(10);
  const [totalPages, setTotalPages] = useState(1);
  const [sortBy, setSortBy] = useState('student_name');
  const [sortOrder, setSortOrder] = useState('asc');
  const [studentNameFilter, setStudentNameFilter] = useState('');
  const [subjectFilter, setSubjectFilter] = useState('');
  const [gradeMinFilter, setGradeMinFilter] = useState('');
  const [gradeMaxFilter, setGradeMaxFilter] = useState('');
  const [error, setError] = useState(null);
  const fileInputRef = useRef();
  const eventSourceRef = useRef(null);

  const handleFileInputClick = () => {
    fileInputRef.current.click();
  };

  const formatFileSize = (sizeInBytes) => {
    if (sizeInBytes < 1024) return `${sizeInBytes} B`;
    if (sizeInBytes < 1024 * 1024) return `${(sizeInBytes / 1024).toFixed(2)} KB`;
    if (sizeInBytes < 1024 * 1024 * 1024)
      return `${(sizeInBytes / (1024 * 1024)).toFixed(2)} MB`;
    return `${(sizeInBytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
  };

  // Initialize SSE connection
  useEffect(() => {
    if (files.length > 0 && !eventSourceRef.current) {
      console.log('Establishing SSE connection...');
      
      eventSourceRef.current = new EventSource('http://localhost:8080/progress/sse');
      
      eventSourceRef.current.onmessage = (event) => {
        console.log('SSE message received:', event.data);
        
        try {
          const data = JSON.parse(event.data);
          
          // Calculate processing percentage based on Processed/TotalRecords
          const processingPercentage = Math.round(
            (data.Processed / data.TotalRecords) * 100
          );

          setProgress(prevProgress => {
            const newProgress = {
              ...prevProgress,
              [data.FileName]: {
                uploadProgress: prevProgress[data.FileName]?.uploadProgress || 100,
                processingProgress: processingPercentage
              }
            };

            // Update total progress
            setTotalProgress(calculateTotalProgress(newProgress));

            return newProgress;
          });

          // Handle completed processing
          if (processingPercentage === 100) {
            setFiles(prevFiles => 
              prevFiles.map(file => 
                file.name === data.FileName ? { ...file, completed: true } : file
              )
            );
            setUploadedFiles(prevUploaded => [
              ...prevUploaded,
              { 
                name: data.FileName, 
                size: formatFileSize(files.find(f => f.name === data.FileName)?.size || 0) 
              }
            ]);
          }
        } catch (error) {
          console.error('Error processing SSE message:', error);
        }
      };

      eventSourceRef.current.onerror = (error) => {
        console.error('SSE Error:', error);
        if (eventSourceRef.current) {
          eventSourceRef.current.close();
          eventSourceRef.current = null;
        }
      };
    }

    return () => {
      if (eventSourceRef.current) {
        console.log('Closing SSE connection');
        eventSourceRef.current.close();
        eventSourceRef.current = null;
      }
    };
  }, [files]);  

  const calculateTotalProgress = (progressData) => {
    const activeFiles = Object.keys(progressData);
    if (activeFiles.length === 0) return { upload: 0, processing: 0 };

    // Check if all files are at 100% upload
    const allUploadsComplete = activeFiles.every(
      filename => progressData[filename].uploadProgress === 100
    );

    const totalProcessing = activeFiles.reduce((sum, filename) => 
      sum + (progressData[filename].processingProgress || 0), 0
    );

    return {
      upload: allUploadsComplete ? 100 : Math.round(
        activeFiles.reduce((sum, filename) => 
          sum + (progressData[filename].uploadProgress || 0), 0
        ) / activeFiles.length
      ),
      processing: Math.round(totalProcessing / activeFiles.length)
    };
  };

  const handleFileUpload = async (event) => {
    const selectedFiles = Array.from(event.target.files);
    if (selectedFiles.length === 0) return;

    const invalidFiles = selectedFiles.filter(
      (file) => !file.name.toLowerCase().endsWith('.csv')
    );

    if (invalidFiles.length > 0) {
      alert('Only CSV files are allowed!');
      return;
    }

    setShowProgress(true);

    const newProgress = { ...progress };
    selectedFiles.forEach(file => {
      newProgress[file.name] = { uploadProgress: 0, processingProgress: 0 };
    });
    setProgress(newProgress);
    setTotalProgress(calculateTotalProgress(newProgress));

    const formData = new FormData();
    selectedFiles.forEach((file) => {
      formData.append('files', file);
      setFiles((prevFiles) => [
        ...prevFiles,
        { name: file.name, size: file.size, completed: false }
      ]);
    });

    try {
      await axios.post('http://localhost:8080/upload', formData, {
        headers: { 'Content-Type': 'multipart/form-data' },
        onUploadProgress: (progressEvent) => {
          const percentCompleted = Math.round(
            (progressEvent.loaded * 100) / progressEvent.total
          );
          
          setProgress(prevProgress => {
            const newProgress = { ...prevProgress };
            selectedFiles.forEach(file => {
              newProgress[file.name] = {
                ...newProgress[file.name],
                uploadProgress: percentCompleted
              };
            });
            
            setTotalProgress(calculateTotalProgress(newProgress));
            return newProgress;
          });
        },
      });

      // Set upload progress to 100% after successful upload
      setProgress(prevProgress => {
        const newProgress = { ...prevProgress };
        selectedFiles.forEach(file => {
          newProgress[file.name] = {
            ...newProgress[file.name],
            uploadProgress: 100
          };
        });
        
        setTotalProgress(calculateTotalProgress(newProgress));
        return newProgress;
      });

    } catch (error) {
      console.error('Error uploading files:', error);
      setError('Failed to upload files. Please try again.');
      
      // Clean up failed uploads
      setFiles(prevFiles => 
        prevFiles.filter(f => !selectedFiles.some(sf => sf.name === f.name))
      );
      
      setProgress(prevProgress => {
        const newProgress = { ...prevProgress };
        selectedFiles.forEach(file => {
          delete newProgress[file.name];
        });
        setTotalProgress(calculateTotalProgress(newProgress));
        return newProgress;
      });
    }

    fileInputRef.current.value = '';
  };

  // Fetch students data
  const fetchStudents = async () => {
    try {
      const params = {
        page,
        limit,
        sort_by: sortBy,
        sort_order: sortOrder,
        student_name: studentNameFilter,
        subject: subjectFilter,
        grade_min: gradeMinFilter,
        grade_max: gradeMaxFilter,
      };

      const response = await axios.get('http://localhost:8080/students', { params });
      setStudents(response.data.data);
      setTotalPages(response.data.totalPages);
    } catch (error) {
      console.error('Error fetching students:', error);
      setError('Failed to fetch students. Please try again later.');
    }
  };

  useEffect(() => {
    fetchStudents();
  }, [page, limit, sortBy, sortOrder, studentNameFilter, subjectFilter, gradeMinFilter, gradeMaxFilter]);

  const handlePageChange = (newPage) => {
    setPage(newPage);
  };

  const handleSort = (column) => {
    if (sortBy === column) {
      setSortOrder(sortOrder === 'asc' ? 'desc' : 'asc');
    } else {
      setSortBy(column);
      setSortOrder('asc');
    }
  };

  return (
    <div className="upload-box">
      <p>Upload your CSV files</p>
      <form>
        <input
          className="file-input"
          type="file"
          name="files"
          hidden
          ref={fileInputRef}
          multiple
          accept=".csv"
          onChange={handleFileUpload}
        />
        <div className="icon" onClick={handleFileInputClick}>
          <img src="" alt="Upload Icon" />
        </div>
      </form>

      {/* Total Progress */}
      {showProgress && files.length > 0 && (
        <div className="progress-container">
          <h3>Total Progress</h3>
          <div className="total-progress">
            <div className="progress-bar-container">
              <label>Total Upload Progress:</label>
              <progress 
                value={totalProgress.upload} 
                max="100"
              ></progress>
              <span>{totalProgress.upload}%</span>
            </div>
            <div className="progress-bar-container">
              <label>Total Processing Progress:</label>
              <progress 
                value={totalProgress.processing} 
                max="100"
              ></progress>
              <span>{totalProgress.processing}%</span>
            </div>
          </div>

          <h3>Individual File Progress</h3>
          {files.map((file) => (
            <div key={file.name} className={`file-progress ${file.completed ? 'completed' : ''}`}>
              <p>{file.name} - {formatFileSize(file.size)} {file.completed && "(Completed)"}</p>
              
              <div className="progress-bar-container">
                <label>Upload Progress:</label>
                <progress 
                  value={progress[file.name]?.uploadProgress || 0} 
                  max="100"
                ></progress>
                <span>{progress[file.name]?.uploadProgress || 0}%</span>
              </div>

              <div className="progress-bar-container">
                <label>Processing Progress:</label>
                <progress 
                  value={progress[file.name]?.processingProgress || 0} 
                  max="100"
                ></progress>
                <span>{progress[file.name]?.processingProgress || 0}%</span>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Error Display */}
      {error && (
        <div className="error-message">
          {error}
          <button onClick={() => setError(null)}>Dismiss</button>
        </div>
      )}

      {/* Students Table */}
      <div className="students-table">
        <h3>Students Data</h3>
        <div className="filters">
          <input
            type="text"
            placeholder="Filter by student name"
            value={studentNameFilter}
            onChange={(e) => setStudentNameFilter(e.target.value)}
          />
          <input
            type="text"
            placeholder="Filter by subject"
            value={subjectFilter}
            onChange={(e) => setSubjectFilter(e.target.value)}
          />
          <input
            type="number"
            placeholder="Min grade"
            value={gradeMinFilter}
            onChange={(e) => setGradeMinFilter(e.target.value)}
          />
          <input
            type="number"
            placeholder="Max grade"
            value={gradeMaxFilter}
            onChange={(e) => setGradeMaxFilter(e.target.value)}
          />
        </div>
        <table>
          <thead>
            <tr>
              <th onClick={() => handleSort('student_id')}>Student ID</th>
              <th onClick={() => handleSort('student_name')}>Student Name</th>
              <th onClick={() => handleSort('subject')}>Subject</th>
              <th onClick={() => handleSort('grade')}>Grade</th>
            </tr>
          </thead>
          <tbody>
            {students.map((student) => (
              <tr key={student.StudentID}>
                <td>{student.StudentID}</td>
                <td>{student.StudentName}</td>
                <td>{student.Subject}</td>
                <td>{student.Grade}</td>
              </tr>
            ))}
          </tbody>
        </table>
        <div className="pagination">
          <button 
            onClick={() => handlePageChange(page - 1)} 
            disabled={page === 1}
          >
            Previous
          </button>
          <span>Page {page} of {totalPages}</span>
          <button 
            onClick={() => handlePageChange(page + 1)} 
            disabled={page === totalPages}
          >
            Next
          </button>
        </div>
      </div>
    </div>
  );
}

export default App;