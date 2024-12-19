-- Drop tables in reverse order to avoid foreign key constraint issues
DROP TABLE IF EXISTS Report;
DROP TABLE IF EXISTS Rental_Services;
DROP TABLE IF EXISTS Service;
DROP TABLE IF EXISTS Log;
DROP TABLE IF EXISTS Transaction;
DROP TABLE IF EXISTS Rental_History;
DROP TABLE IF EXISTS Admin;
DROP TABLE IF EXISTS Computer;
DROP TABLE IF EXISTS Customer;

-- 1. Customer Table
CREATE TABLE Customer (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    username VARCHAR(100) UNIQUE NOT NULL,
    email VARCHAR(100) UNIQUE NOT NULL,
    password VARCHAR(255) NOT NULL,
    jwt_token VARCHAR(255),
    wallet DOUBLE PRECISION DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 2. Computer Table
CREATE TABLE Computer (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    type VARCHAR(100) NOT NULL,
    isAvailable BOOLEAN DEFAULT TRUE,
    hourly_rate INTEGER NOT NULL,
    last_maintenance_date TIMESTAMP
);

-- 3. Admin Table
CREATE TABLE Admin (
    id SERIAL PRIMARY KEY,
    username VARCHAR(100) UNIQUE NOT NULL,
    password VARCHAR(255) NOT NULL,
    role VARCHAR(100) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    jwt_token VARCHAR(255)
);

-- 4. Rental_History Table
CREATE TABLE Rental_History (
    id SERIAL PRIMARY KEY,
    customer_id INTEGER NOT NULL,
    computer_id INTEGER NOT NULL,
    admin_id INTEGER,
    rental_start_time TIMESTAMP NOT NULL,
    rental_end_time TIMESTAMP NOT NULL,
    total_cost INTEGER NOT NULL,
    booking_status VARCHAR(100) DEFAULT 'Pending',
    FOREIGN KEY (customer_id) REFERENCES Customer(id),
    FOREIGN KEY (computer_id) REFERENCES Computer(id),
    FOREIGN KEY (admin_id) REFERENCES Admin(id)
);

-- 5. Transaction Table
CREATE TABLE Transaction (
    id SERIAL PRIMARY KEY,
    customer_id INTEGER NOT NULL,
    transaction_type VARCHAR(100) NOT NULL,
    amount DOUBLE PRECISION NOT NULL,
    transaction_method VARCHAR(100),
    transaction_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(100) NOT NULL,
    order_id VARCHAR(100) UNIQUE,
    payment_url VARCHAR(255),
    FOREIGN KEY (customer_id) REFERENCES Customer(id)
);

-- 6. Log Table
CREATE TABLE Log (
    id SERIAL PRIMARY KEY,
    description varchar(250) 
);

-- 7. Service Table
CREATE TABLE Service (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    price DOUBLE PRECISION NOT NULL,
    description VARCHAR(250),
    quantity INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 8. Rental_Services Table
CREATE TABLE Rental_Services (
    id SERIAL PRIMARY KEY,
    rental_history_id INTEGER,
    service_id INTEGER NOT NULL,
    quantity INTEGER DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (rental_history_id) REFERENCES Rental_History(id) ON DELETE CASCADE,
    FOREIGN KEY (service_id) REFERENCES Service(id) ON DELETE CASCADE
);

-- 9. Report Table
CREATE TABLE Report (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    report_type VARCHAR(100) NOT NULL,
    start_date TIMESTAMP NOT NULL,
    end_date TIMESTAMP NOT NULL,
    total_transactions INTEGER DEFAULT 0,
    total_revenue DOUBLE PRECISION DEFAULT 0,
    total_rentals INTEGER DEFAULT 0,
    top_services VARCHAR(250),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (admin_id) REFERENCES Admin(id)
);

-- to insert metadata column into transaction
ALTER TABLE transaction ADD COLUMN metadata JSONB DEFAULT '{}'::JSONB;

-- Insert customer data
INSERT INTO Customer (name, username, email, password, wallet)
VALUES 
('John Doe', 'johndoe', 'john@example.com', 'hashed_password_1', 100000),
('Jane Smith', 'janesmith', 'jane@example.com', 'hashed_password_2', 150000),
('Bob Brown', 'bobbrown', 'bob@example.com', 'hashed_password_3', 200000);

-- Insert computer data
INSERT INTO Computer (name, type, isAvailable, hourly_rate, last_maintenance_date)
VALUES 
('PC-001', 'Gaming', TRUE, 20000, '2024-12-01 10:00:00'),
('PC-002', 'Office', TRUE, 10000, '2024-12-05 12:00:00'),
('PC-003', 'Browsing', FALSE, 5000, '2024-12-10 09:00:00'),
('PC-004', 'Gaming', TRUE, 25000, '2024-12-15 11:00:00'),
('PC-005', 'Office', TRUE, 12000, '2024-12-12 08:00:00'),
('PC-006', 'Browsing', TRUE, 7000, '2024-12-09 15:00:00'),
('PC-007', 'Gaming', FALSE, 22000, '2024-12-01 14:00:00'),
('PC-008', 'Office', TRUE, 11000, '2024-12-11 09:30:00'),
('PC-009', 'Browsing', TRUE, 6000, '2024-12-14 16:00:00'),
('PC-010', 'Gaming', TRUE, 30000, '2024-12-16 10:00:00'),
('PC-011', 'Office', TRUE, 15000, '2024-12-13 13:00:00'),
('PC-012', 'Browsing', TRUE, 8000, '2024-12-08 10:45:00'),
('PC-013', 'Gaming', FALSE, 28000, '2024-12-04 18:00:00'),
('PC-014', 'Office', TRUE, 13000, '2024-12-10 14:00:00'),
('PC-015', 'Browsing', TRUE, 5500, '2024-12-07 12:00:00'),
('PC-016', 'Gaming', TRUE, 24000, '2024-12-17 09:00:00'),
('PC-017', 'Office', FALSE, 14000, '2024-12-03 11:00:00'),
('PC-018', 'Browsing', TRUE, 7500, '2024-12-06 10:00:00'),
('PC-019', 'Gaming', TRUE, 26000, '2024-12-02 12:00:00'),
('PC-020', 'Office', TRUE, 12500, '2024-12-15 08:00:00');

-- Insert admin data
INSERT INTO Admin (username, password, role)
VALUES 
('admin1', 'hashed_admin_password_1', 'Manager'),
('admin2', 'hashed_admin_password_2', 'Operator');

-- Insert service table
INSERT INTO Service (name, price, description, quantity)
VALUES 
('Printing', 2500, 'Black and white printing per page', 100),
('Snacks', 5000, 'Pack of chips or cookies', 50),
('Drinks', 3000, 'Cold or hot beverages', 75),
('Photocopying', 1500, 'Black and white photocopy per page', 200),
('Laminating', 5000, 'Laminating service per page', 30),
('Binding', 10000, 'Binding service for documents', 20),
('Internet Access', 2000, 'Access to high-speed internet per hour', 150),
('USB Drive', 50000, '16GB USB Drive for sale', 10),
('Headphones', 15000, 'Headphones for rental', 20),
('Keyboard', 10000, 'Keyboard for rental', 15),
('Mouse', 5000, 'Mouse for rental', 25);

-- Insert rental_history table
INSERT INTO Rental_History (customer_id, computer_id, admin_id, rental_start_time, rental_end_time, total_cost)
VALUES 
(1, 1, 1, '2024-12-18 10:00:00', '2024-12-18 12:00:00', 40000),
(2, 2, 2, '2024-12-18 11:00:00', '2024-12-18 13:00:00', 20000);

-- Insert rental services table
INSERT INTO Rental_Services (rental_history_id, service_id, quantity)
VALUES 
(1, 1, 3), -- 3 printings during rental session 1
(1, 2, 1), -- 1 snack
(2, 3, 2); -- 2 drinks during rental session 2

-- Insert into transactions table
INSERT INTO Transaction (customer_id, transaction_type, amount, transaction_method, status)
VALUES 
(1, 'Top-up', 100000, 'Credit Card', 'Completed'),
(2, 'Payment', 40000, 'Cash', 'Completed'),
(3, 'Top-up', 200000, 'Stripe', 'Completed');

-- Insert into reports table
INSERT INTO Report (admin_id, report_type, start_date, end_date, total_transactions, total_revenue, total_rentals, top_services)
VALUES 
(1, 'Daily Revenue Report', '2024-12-18 00:00:00', '2024-12-18 23:59:59', 3, 340000, 2, 'Printing, Snacks'),
(2, 'Service Usage Report', '2024-12-18 00:00:00', '2024-12-18 23:59:59', 0, 0, 0, 'Snacks, Drinks');
